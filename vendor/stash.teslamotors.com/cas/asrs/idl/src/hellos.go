package asrs

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/golang/protobuf/ptypes"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Connection interface {
	Send(*Hello) error
	Recv() (*Hello, error)
}

// PeerTracker can be used to access the notification channel which notifies change in state,
// and access to state.
type PeerTracker struct {
	Notify chan struct{}
	// line and name
	line string
	name string
	// periodMultiplier is a multipl of timeout, used to determine who often we launch a hello (echo request).
	period time.Duration
	// timeout is how long we wait for a response to a hello.
	timeout time.Duration
	// connection used to send and receive hellos
	conn    Connection
	metrics *HelloMetricsHolder
	// logger used for peer tracker
	*zap.SugaredLogger
	// Lock used to protect access to state and lastChange via GetState
	sync.RWMutex
	lastChange time.Time
	state      bool
	discovered string
}

// PeerTracker.GetState returns the current state as determined by keepalives,
// indicating state and when the state last changed or creation time if state has
// not changed since. GetState also returns the name reported by the remote endpoint.
func (p *PeerTracker) GetState() (bool, time.Time, string) {
	p.RLock()
	defer p.RUnlock()
	return p.state, p.lastChange, p.discovered
}

const (
	// direction
	DirRx = "rx"
	DirTx = "tx"
	// result
	ResOk         = "ok"
	ResSendFailed = "sendFailed"

	// Hello Specific results
	ResHelloWithUnknownNonce = "helloWithUnexpectedNonce"
	ResHelloTimeoutOnReply   = "helloTimedUutOnReply"
	// echo
	HelloEchoRequest = "helloEchoRequest"
	HelloEchoReply   = "helloEchoReply"
	// peer state derived from hellos
	HelloReachable   = "helloReachable"
	HelloUnreachable = "helloUnreachable"
)

type HelloMetricsHolder struct {
	HellosCounter            *prometheus.CounterVec
	HellosStateChangeCounter *prometheus.CounterVec
	HelloStateGauge          *prometheus.GaugeVec
}

// NewPeerTracker is used to create a tracker which uses hellos to determine state of peer and also respond
// to the peers hellos.
func NewPeerTracker(
	conn Connection, line, name string, timeout time.Duration, periodMultiplier uint,
	metrics *HelloMetricsHolder, logger *zap.SugaredLogger) *PeerTracker {

	logger = logger.With("pkg", "peerTracker")

	if timeout == 0 || periodMultiplier == 0 {
		periodMultiplier = 0
		timeout = 0
		logger.Warn("peerTracker_hellos_will_not_be_sent_from_this_end_(but_will_respond_to_requests)")
	} else {
		if periodMultiplier == 1 {
			logger.Warn("peerTracker_increasing_periodMultiplier_to_be_a_multiple_of_timeout_period")
			periodMultiplier++
		}
	}

	return &PeerTracker{
		Notify:        make(chan struct{}, 1),
		line:          line,
		name:          name,
		period:        time.Duration(periodMultiplier) * timeout,
		timeout:       timeout,
		conn:          conn,
		metrics:       metrics,
		SugaredLogger: logger,
		RWMutex:       sync.RWMutex{},
		lastChange:    time.Now(),
		state:         false,
	}
}

func (p *PeerTracker) Start(ctx context.Context, remoteEnd string, remoteName string) {
	defer close(p.Notify)

	// Log and debug anon function
	trace := func(labels []string, msg string, err error, hello *Hello) {
		if p.metrics != nil {
			p.metrics.HellosCounter.WithLabelValues(labels...).Inc()
		}
		if msg != "" {
			logFields := getHelloZapFields(hello)
			if err != nil {
				logFields = append(logFields, "error", err)
				p.Errorw(msg, logFields...)
			} else {
				p.Debugw(msg, logFields...)
			}
		}
	}

	//
	// Set up reader
	rxHelloFeed := make(chan *Hello)

	// Let's kick off a go routine to handle receiving messages.
	go func(ctx context.Context, feed chan *Hello) {
		defer close(feed)
		for {
			// Recv() will error out when channel is closed. This is how we stop listening.
			hello, err := p.conn.Recv()
			if err != nil {

				st, ok := status.FromError(err)
				if err == io.EOF || (ok && st.Code() == codes.Canceled) {
					p.Debugw("peerTracker_reader_goroutine_closing_down_stream_closed", ""+
						"remote", remoteEnd, "peer", remoteName)
				} else {
					p.Errorw("peerTracker_reader_goroutine_closing_down_stream_error",
						"remote", remoteEnd, "peer", remoteName,
						"error", err)
				}
				return
			}
			select {
			case feed <- hello:
			case <-ctx.Done():
				p.Debugw("peerTracker_reader_goroutine_closing_down_context_done", "remote",
					remoteEnd, "reason", ctx.Err())
				return
			}
		}
	}(ctx, rxHelloFeed)

	var nonce int64

	var launchTimer *time.Timer
	var helloPeriodChan <-chan time.Time
	if p.period != 0 {
		// We use timer instead of ticker to set the first launch to immediate
		launchTimer = time.NewTimer(1 * time.Millisecond)
		helloPeriodChan = launchTimer.C
	}

	// Start out nil and not ticking.
	var timeout <-chan time.Time

outerLoop:
	for {
		select {

		case <-timeout:
			// hmm... we found a hello for which we have not got a Response in time
			trace([]string{DirRx, ResHelloTimeoutOnReply, remoteName, HelloEchoReply},
				"peerTracker_echo_request_never_received",
				fmt.Errorf("gave up waiting for nonce %v from %s", nonce, remoteEnd), nil)
			timeout = nil
			nonce = 0
			p.evaluateStateChange(remoteName, false)

		case <-helloPeriodChan:

			// If hello period is configured this channel will tick. Set up a new nonce, and launch.
			nonce = rand.Int63()
			hello := &Hello{
				Conversation: p.buildConversationHeader(MessageIDNone),
				EchoRequest:  true,
				Nonce:        nonce,
			}
			err := p.conn.Send(hello)
			if err != nil {
				trace([]string{DirTx, ResSendFailed, remoteName, HelloEchoRequest},
					"peerTracker_sent_echo_request", err, hello)
				break outerLoop
			}
			trace([]string{DirTx, ResOk, remoteName, HelloEchoRequest}, "", err, hello)

			// Setup timeout... (on the expensive side - we could reuse timer... but we don't have many connections.
			timeout = time.NewTimer(p.timeout).C

			// Set up timer for next echo request
			launchTimer.Reset(p.period)

		case hello, ok := <-rxHelloFeed:
			if !ok {
				// reader function logged on exit
				break outerLoop
			}

			if hello.EchoRequest {

				trace([]string{DirRx, ResOk, remoteName, HelloEchoRequest}, "", nil, hello)

				hello.EchoRequest = false
				hello.Conversation = p.buildConversationHeader(hello.Conversation.Id())
				err := p.conn.Send(hello)
				if err != nil {
					trace([]string{DirTx, ResSendFailed, remoteName, HelloEchoReply},
						"peerTracker_txed_echo_reply", err, hello)
					break outerLoop
				}
				trace([]string{DirTx, ResOk, remoteName, HelloEchoReply}, "", err, hello)
				break
			}

			// We have a reply. Check if it matches nonce?
			rxNonce := hello.Nonce
			if rxNonce != 0 && rxNonce == nonce {
				trace([]string{DirRx, ResOk, remoteName, HelloEchoReply}, "", nil, hello)
				timeout = nil
				nonce = 0 // stop waiting for it
				p.evaluateStateChange(remoteName, true)
			} else {
				trace([]string{DirRx, ResHelloWithUnknownNonce, remoteName, HelloEchoReply},
					"peerTracker_rxed_echo_reply_unexpected", nil, hello)
			}
		}
	}

	// We expect hello feed to close too when we're on the way out... let's wait for it and make sure.
	p.Debugw("peerTracker_main_handler_closing_down_waiting_for_reader", "remote", remoteEnd)
	for {
		_, ok := <-rxHelloFeed
		if !ok {
			break
		}
	}
	// Issue final unreachable...
	p.evaluateStateChange(remoteName, false)
}

func (p *PeerTracker) evaluateStateChange(remoteEnd string, ok bool) {
	// evaluate state is always called from the goroutine where state can be changed so we only need to lock
	// if we are about to make a change. Notification is non blocking.
	if p.state != ok {
		p.Lock()
		p.state = ok
		p.lastChange = time.Now()
		p.discovered = remoteEnd
		p.Unlock()
		if p.metrics != nil {
			if ok {
				p.metrics.HellosStateChangeCounter.WithLabelValues(HelloReachable, remoteEnd).Inc()
				p.metrics.HelloStateGauge.WithLabelValues(remoteEnd).Set(float64(1))
			} else {
				p.metrics.HellosStateChangeCounter.WithLabelValues(HelloUnreachable, remoteEnd).Inc()
				p.metrics.HelloStateGauge.WithLabelValues(remoteEnd).Set(float64(0))
			}
		}
		p.Infow("peerTracker_notifying_owner_of_change_of_state",
			"remote", remoteEnd, "state", ok)
		select {
		case p.Notify <- struct{}{}:
		default:
			p.Infow("peerTracker_notification_of_change_of_state_not_required_pending_notification",
				"remote", remoteEnd, "state", ok)
			// Notification pending, no need to issue another one. Just proceed.
		}
	}
}

func (p *PeerTracker) buildConversationHeader(mid MessageId) *Conversation {
	return BuildConversationHeader(p.line, p.name, mid)
}

func getConversationZapFields(c *Conversation) []interface{} {
	if c == nil {
		return []interface{}{}
	}

	// nil timestamp handled correctly in called function
	ots, _ := ptypes.Timestamp(c.Originated)

	return []interface{}{"origin", c.Origin, "line", c.Line, "originated", ots}
}

func getHelloZapFields(h *Hello) []interface{} {
	if h == nil {
		return []interface{}{}
	}
	return append(getConversationZapFields(h.Conversation),
		"echorequest", h.EchoRequest,
		"nonce", h.Nonce)
}

func HelloRemoteAddressAndNameFromContext(ctx context.Context) (string, string) {
	remoteEnd := "unidentified"
	pr, ok := peer.FromContext(ctx)
	if ok {
		remoteEnd = pr.Addr.String()
	}

	remoteName := "undiscovered"
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		values := md.Get("name")
		if len(values) > 0 {
			remoteName = values[0]
		}
	}
	return remoteEnd, remoteName
}
