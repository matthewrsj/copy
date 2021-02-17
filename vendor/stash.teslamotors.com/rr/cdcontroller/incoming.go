package cdcontroller

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
	asrsapi "stash.teslamotors.com/cas/asrs/idl/src"
)

func handleIncomingLoad(
	g asrsapi.Terminal_LoadOperationsServer,
	lg *zap.SugaredLogger,
	prodAM, testAM *AisleManager,
	aisles map[string]*Aisle,
	lo *asrsapi.LoadOperation,
) error {
	logger := lg.With(
		"location", lo.GetLocation().GetCmFormat().GetEquipmentId(),
		"trays", lo.GetTray().GetTrayId(),
	)

	switch lo.GetState().GetState() {
	case asrsapi.LoadOperationState_PreparedForDelivery:
		logger.Info("got PreparedForDelivery")

		if lo.GetState().GetStateType() != asrsapi.StateType_Desired {
			logger.Info("not Desired, ignoring")
			return nil
		}

		logger.Info("PreparedForDelivery Desired")

		aisleLocation := lo.GetLocation().GetCmFormat().GetEquipment()

		// if aisleLocation is empty perform the initial load
		if strings.Trim(aisleLocation, "0") == "" {
			logger.Info("no aisle location, routing to aisle")
			return handleInitialLoad(g, logger, prodAM, testAM, aisles, lo)
		}

		// if aisleLocation is populated perform the tower load
		logger.Infow("aisle location", "aisle", aisleLocation)

		return handleTowerLoad(g, logger, aisles, lo)
	case asrsapi.LoadOperationState_Loaded:
		logger.Info("got Loaded")

		if lo.GetState().GetStateType() != asrsapi.StateType_Current {
			logger.Info("not Current, ignoring")
			return nil
		}

		logger.Info("Loaded Current")

		return handleTowerLoaded(logger, g, aisles, lo)
	default:
		logger.Warnw("unhandled GetState().GetState()", "state", lo.GetState().GetState())
	}

	return nil
}

const _nonProdPrefix = "test_"

func selectWithRoundRobin(logger *zap.SugaredLogger, prodAM, testAM *AisleManager, aisles map[string]*Aisle, lo *asrsapi.LoadOperation) (string, error) {
	need := len(lo.GetTray().GetTrayId())
	logger.Debugw("need space for trays for round robin", "need", need)

	// determine which aisle manager to use
	am := prodAM
	if strings.HasPrefix(strings.ToLower(lo.GetRecipe().GetStep()), strings.ToLower(_nonProdPrefix)) {
		am = testAM
	}

	var aislePicked string

	for i := 0; i < len(am.OpenAisles()); i++ {
		aisleName := am.GetNextAisleName()
		if aisleName == "" {
			return "", errors.New("unable to find next aisle name")
		}

		logger.Infow("checking availability for aisle for initial place", "aisle", aisleName)

		aisle, ok := aisles[aisleName]
		if !ok {
			logger.Errorw("invalid aisle name", "name", aisleName)
			continue
		}

		// remove spaces and _nonProdPrefix then check if it's a commissioning tray
		// we've already routed to a non-production aisle at this point if it has _nonProdPrefix
		stepName := strings.TrimPrefix(strings.TrimSpace(strings.Split(lo.GetRecipe().GetStep(), " - ")[0]), _nonProdPrefix)
		isCommissionRecipe := strings.HasPrefix(stepName, CommissionSelfTestRecipeName)
		aisle.avail = 0

		var (
			twg sync.WaitGroup
			mx  sync.Mutex
		)

		twg.Add(len(aisle.Towers))

		for _, tower := range aisle.Towers {
			go func(tower *Tower) {
				defer twg.Done()

				availFunc := tower.getAvailability
				if isCommissionRecipe {
					// has CommissionSelfTestRecipeName, so check for availability for commissioning
					// instead of regular operation.
					availFunc = tower.getAvailabilityForCommissioning
				}

				available, err := availFunc()
				if err != nil {
					logger.Errorw("get tower availability", "error", err)
					return
				}

				mx.Lock()
				aisle.avail += available.GetAvail() // not atomic
				mx.Unlock()
			}(tower)
		}

		twg.Wait()

		logger.Infow("aisle has available fixtures", "aisle", aisleName, "available", aisle.avail)

		if aisle.avail >= need {
			aislePicked = aisleName
			break
		}

		logger.Infow("not enough fixtures available, checking next aisle", "aisle", aisleName)
	}

	if aislePicked == "" {
		return "", errors.New("not enough fixtures available")
	}

	return aislePicked, nil
}

func selectMaxAvailable(logger *zap.SugaredLogger, prodAM, testAM *AisleManager, aisles map[string]*Aisle, lo *asrsapi.LoadOperation) (string, error) {
	need := len(lo.GetTray().GetTrayId())
	logger.Debugw("need space for trays", "need", need)

	// determine which aisle manager to use
	am := prodAM
	if strings.HasPrefix(strings.ToLower(lo.GetRecipe().GetStep()), strings.ToLower(_nonProdPrefix)) {
		am = testAM
	}

	var (
		maxAvailAisleName string
		maxAvailAisleNum  int
	)

	for i := 0; i < len(am.OpenAisles()); i++ {
		aisleName := am.GetNextAisleName()
		if aisleName == "" {
			return "", errors.New("unable to find next aisle name")
		}

		logger.Infow("checking availability for aisle for initial place", "aisle", aisleName)

		aisle, ok := aisles[aisleName]
		if !ok {
			logger.Errorw("invalid aisle name", "name", aisleName)
			continue
		}

		// remove spaces and _nonProdPrefix then check if it's a commissioning tray
		// we've already routed to a non-production aisle at this point if it has _nonProdPrefix
		stepName := strings.TrimPrefix(strings.TrimSpace(strings.Split(lo.GetRecipe().GetStep(), " - ")[0]), _nonProdPrefix)
		isCommissionRecipe := strings.HasPrefix(stepName, CommissionSelfTestRecipeName)
		aisle.avail = 0

		var (
			twg sync.WaitGroup
			mx  sync.Mutex
		)

		twg.Add(len(aisle.Towers))

		for _, tower := range aisle.Towers {
			go func(tower *Tower) {
				defer twg.Done()

				availFunc := tower.getAvailability
				if isCommissionRecipe {
					// has CommissionSelfTestRecipeName, so check for availability for commissioning
					// instead of regular operation.
					availFunc = tower.getAvailabilityForCommissioning
				}

				available, err := availFunc()
				if err != nil {
					logger.Errorw("get tower availability", "error", err)
					return
				}

				mx.Lock()
				aisle.avail += available.GetAvail() // not atomic
				mx.Unlock()
			}(tower)
		}

		twg.Wait()

		logger.Infow("aisle has available fixtures", "aisle", aisleName, "available", aisle.avail)

		if aisle.avail >= maxAvailAisleNum {
			maxAvailAisleName = aisleName
			maxAvailAisleNum = aisle.avail
		}
	}

	if maxAvailAisleName == "" || maxAvailAisleNum < need {
		return "", errors.New("not enough fixtures available")
	}

	return maxAvailAisleName, nil
}

func handleInitialLoad(
	g asrsapi.Terminal_LoadOperationsServer,
	logger *zap.SugaredLogger,
	prodAM, testAM *AisleManager,
	aisles map[string]*Aisle,
	lo *asrsapi.LoadOperation,
) error {
	// whether or not we place into aisle, we need to set this to Current
	lo.GetState().StateType = asrsapi.StateType_Current

	// select algorithm based on if this is a cycling/demo tray and whether or not it is a commissioning tray
	// If this is just a non-production tray route based on the round robin only
	// If this is a commissioning tray route by selecting the maximum available as normal
	// If this is a production tray route based on availability as normal
	stepName := strings.TrimSpace(strings.Split(lo.GetRecipe().GetStep(), " - ")[0])
	// remove test_ prefix to check for commission tray status
	isCommissionRecipe := strings.HasPrefix(strings.TrimPrefix(stepName, _nonProdPrefix), CommissionSelfTestRecipeName)
	isNonProdTray := strings.HasPrefix(stepName, _nonProdPrefix)

	selector := selectMaxAvailable
	if !isCommissionRecipe && isNonProdTray {
		// this is a normal non-production tray, route via round-robin
		selector = selectWithRoundRobin
	}

	aislePicked, err := selector(logger, prodAM, testAM, aisles, lo)
	if err != nil {
		// no aisle found with enough availability, route back to current location
		// keep the location the same
		return backoff.Retry(func() error {
			return g.Send(lo)
		}, backoff.NewExponentialBackOff())
	}

	logger.Infow("routing to aisle", "aisle", aislePicked)
	lo.GetLocation().GetCmFormat().Equipment = aislePicked

	lo.GetLocation().GetCmFormat().EquipmentId, err = replaceAisle(lo.GetLocation().GetCmFormat().GetEquipmentId(), aislePicked)
	if err != nil {
		lo.GetState().Status = &asrsapi.Status{
			Status:      asrsapi.Status_Fault,
			Description: err.Error(),
		}

		_ = g.Send(lo)

		return err
	}

	return backoff.Retry(func() error {
		return g.Send(lo)
	}, backoff.NewExponentialBackOff())
}

func rejectLoad(g asrsapi.Terminal_LoadOperationsServer, lo *asrsapi.LoadOperation, err error) error {
	lo.GetState().Status = &asrsapi.Status{
		Status:      asrsapi.Status_Rejected,
		Description: err.Error(),
	}

	lo.GetState().StateType = asrsapi.StateType_Current

	sErr := g.Send(lo)
	if sErr != nil {
		err = fmt.Errorf("reject tray due to error '%v': %v", err, sErr)
	}

	return err
}

func handleTowerLoad(g asrsapi.Terminal_LoadOperationsServer, lg *zap.SugaredLogger, aisles map[string]*Aisle, lo *asrsapi.LoadOperation) error {
	stepName := strings.TrimPrefix(strings.TrimSpace(strings.Split(lo.GetRecipe().GetStep(), " - ")[0]), _nonProdPrefix)
	isCommissionRecipe := strings.HasPrefix(stepName, CommissionSelfTestRecipeName)

	lg.Infow("handling tower load for recipe", "step", stepName)

	if isCommissionRecipe {
		lg.Debug("commissioning tray, routing to fixture that needs commissioning")

		if err := handleTowerLoadCommissioning(g, lg, aisles, lo); err != nil {
			lg.Errorw("handle tower load for commissioning tray", "error", err)
			return err
		}
	} else {
		lg.Debug("non-commissioning tray, routing to fixture for normal operation")

		if err := handleTowerLoadNormalOperation(g, lg, aisles, lo); err != nil {
			lg.Errorw("handle tower load for normal operation", "error", err)
			return err
		}
	}

	return nil
}

func handleTowerLoadCommissioning(g asrsapi.Terminal_LoadOperationsServer, lg *zap.SugaredLogger, aisles map[string]*Aisle, lo *asrsapi.LoadOperation) error {
	return handleTowerLoadForGetter(g, lg, aisles, lo, func(t *Tower) (*FXRLayout, error) {
		return t.getAvailabilityForCommissioning()
	})
}

func handleTowerLoadNormalOperation(g asrsapi.Terminal_LoadOperationsServer, lg *zap.SugaredLogger, aisles map[string]*Aisle, lo *asrsapi.LoadOperation) error {
	return handleTowerLoadForGetter(g, lg, aisles, lo, func(t *Tower) (*FXRLayout, error) {
		return t.getAvailability()
	})
}

type layoutGetter func(t *Tower) (*FXRLayout, error)

func handleTowerLoadForGetter(g asrsapi.Terminal_LoadOperationsServer, lg *zap.SugaredLogger, aisles map[string]*Aisle, lo *asrsapi.LoadOperation, getter layoutGetter) error {
	loc := lo.GetLocation().GetCmFormat().GetEquipment()
	trays := lo.GetTray().GetTrayId()
	logger := lg.With("num_trays", len(trays))

	aisle, ok := aisles[loc]
	if !ok {
		return rejectLoad(g, lo, fmt.Errorf("invalid aisle %s", loc))
	}

	op, err := getLocation(aisle, lg, trays, 0 /* timesToTry (forever) */, getter)
	if err != nil {
		return rejectLoad(g, lo, fmt.Errorf("find location for tray(s): %v", err))
	}

	if op.sendTwo {
		for i, report := range []availabilityReport{op.front, op.back} {
			loc := lo.GetLocation()
			lo0 := &asrsapi.LoadOperation{
				Conversation: lo.Conversation,
				Tray: &asrsapi.Tray{
					TrayId: []string{trays[i]},
				},
				Location: loc,
				Recipe:   lo.Recipe,
			}

			lo0.State = &asrsapi.LoadOperationStateAndStatus{
				State:     asrsapi.LoadOperationState_PreparedForDelivery,
				StateType: asrsapi.StateType_Current,
				Status: &asrsapi.Status{
					Status: asrsapi.Status_Complete,
				},
			}

			ws, subID := fmt.Sprintf("%02d", report.fixture.Coord.Col), fmt.Sprintf("%02d", report.fixture.Coord.Lvl)
			lo0.GetLocation().GetCmFormat().Workstation = ws
			lo0.GetLocation().GetCmFormat().SubIdentifier = subID

			lo0.GetLocation().GetCmFormat().EquipmentId, err = replaceWorkstationSubID(lo.GetLocation().GetCmFormat().GetEquipmentId(), ws, subID)
			if err != nil {
				return rejectLoad(g, lo, fmt.Errorf("replaceWorkstationSubID: %v", err))
			}

			logger.Info("determined equipment ID", "equipment_id", lo.GetLocation().GetCmFormat().GetEquipmentId())
			logger.Info("sending response to CND")

			if err = backoff.Retry(func() error {
				return g.Send(lo0)
			}, backoff.NewExponentialBackOff()); err != nil {
				return rejectLoad(g, lo, fmt.Errorf("send load operation %v", err))
			}

			if err = reserveOnTower(report.tower, trays[i], lo0.GetLocation().GetCmFormat().GetEquipmentId()); err != nil {
				logger.Warnw("unable to reserve fixture on tower", "error", err)
			}
		}

		return nil
	}

	logger.Infow("location coordinates", "location", op.front.fixture.Coord)

	lo.State = &asrsapi.LoadOperationStateAndStatus{
		State:     asrsapi.LoadOperationState_PreparedForDelivery,
		StateType: asrsapi.StateType_Current,
		Status: &asrsapi.Status{
			Status: asrsapi.Status_Complete,
		},
	}

	ws, subID := fmt.Sprintf("%02d", op.front.fixture.Coord.Col), fmt.Sprintf("%02d", op.front.fixture.Coord.Lvl)
	lo.GetLocation().GetCmFormat().Workstation = ws
	lo.GetLocation().GetCmFormat().SubIdentifier = subID

	lo.GetLocation().GetCmFormat().EquipmentId, err = replaceWorkstationSubID(lo.GetLocation().GetCmFormat().GetEquipmentId(), ws, subID)
	if err != nil {
		return rejectLoad(g, lo, fmt.Errorf("replaceWorkstationSubID: %v", err))
	}

	logger.Infow("determined equipment ID", "equipment_id", lo.GetLocation().GetCmFormat().GetEquipmentId())
	logger.Info("sending response to CND")

	if err = backoff.Retry(func() error {
		return g.Send(lo)
	}, backoff.NewExponentialBackOff()); err != nil {
		return rejectLoad(g, lo, fmt.Errorf("send load operation: %v", err))
	}

	if err = reserveOnTower(op.front.tower, lo.GetTray().GetTrayId()[0], lo.GetLocation().GetCmFormat().GetEquipmentId()); err != nil {
		logger.Warnw("unable to reserve fixture on tower", "error", err)
	}

	if len(trays) > 1 {
		if op.back.fixture == nil {
			logger.Error("back fixture object is nil")
			return fmt.Errorf("back fixture object is nil")
		}

		backWS := fmt.Sprintf("%02d", op.back.fixture.Coord.Col)

		backLoc, err := replaceWorkstationSubID(lo.GetLocation().GetCmFormat().GetEquipmentId(), backWS, subID)
		if err != nil {
			logger.Warn("unable to generate back location sub ID")
		}

		if err = reserveOnTower(op.back.tower, lo.GetTray().GetTrayId()[1], backLoc); err != nil {
			logger.Warnw("unable to reserve fixture on tower", "error", err)
		}
	}

	return nil
}

func handleTowerLoaded(logger *zap.SugaredLogger, g asrsapi.Terminal_LoadOperationsServer, aisles map[string]*Aisle, lo *asrsapi.LoadOperation) error {
	aisleName := lo.GetLocation().GetCmFormat().GetEquipment()

	aisle, ok := aisles[aisleName]
	if !ok {
		return rejectLoad(g, lo, fmt.Errorf("invalid aisle name: %s", aisleName))
	}

	col, err := strconv.Atoi(lo.GetLocation().GetCmFormat().GetWorkstation())
	if err != nil {
		return rejectLoad(g, lo, fmt.Errorf("get column: %v", err))
	}

	lvl, err := strconv.Atoi(lo.GetLocation().GetCmFormat().GetSubIdentifier())
	if err != nil {
		return rejectLoad(g, lo, fmt.Errorf("get level: %v", err))
	}

	for _, tower := range aisle.Towers {
		fxr := tower.FXRs.Get(Coordinates{
			Col: col,
			Lvl: lvl,
		})

		if fxr == nil {
			return rejectLoad(g, lo, fmt.Errorf("invalid location for load complete: %s", lo.GetLocation().GetCmFormat().GetEquipmentId()))
		}

		if fxr.Coord.Col != col {
			continue
		}

		logger.Infow("found fixture to send load to", "fixture", fxr.Coord)

		trays := lo.GetTray().GetTrayId()
		if len(trays) == 0 {
			return rejectLoad(g, lo, errors.New("no trays in request"))
		}

		tID := lo.GetConversation().GetMsgId()

		if err = tower.sendLoad(logger, fxr, trays[0], lo.GetRecipe(), tID); err != nil {
			return rejectLoad(g, lo, fmt.Errorf("send load to tower: %v", err))
		}

		if len(trays) > 1 {
			return rejectLoad(g, lo, fmt.Errorf("two trays in message: %v", err))
		}

		return nil
	}

	return nil
}

type availabilityReport struct {
	tower   *Tower
	layout  *FXRLayout
	numFree int

	fixture *FXR
}

type operation struct {
	front, back availabilityReport
	sendTwo     bool
}

func permErrorIfViolated(timesTried, timesToTry int, err error) error {
	if timesToTry <= 0 || timesTried < timesToTry {
		return err
	}

	return backoff.Permanent(err)
}

type availabilityHandler struct {
	timesTried, timesToTry int
	trays                  []string
	aisle                  *Aisle
	getter                 layoutGetter
	lg                     *zap.SugaredLogger

	first, second availabilityReport
}

func (ah *availabilityHandler) getAvailability() error {
	defer func() { ah.timesTried++ }()

	arChan := make(chan availabilityReport)
	doneChan := make(chan struct{})

	var reports []availabilityReport

	go func() {
		defer close(doneChan)

		for ar := range arChan {
			reports = append(reports, ar)
		}
	}()

	var wg sync.WaitGroup

	wg.Add(len(ah.aisle.Towers))

	for _, tower := range ah.aisle.Towers {
		go func(tower *Tower) {
			defer wg.Done()

			if tower == nil {
				ah.lg.Warn("invalid nil tower")
				return
			}

			available, err := ah.getter(tower)
			if err != nil || available == nil {
				ah.lg.Warnw("get tower availability", "error", err)
				return
			}

			numFree := available.GetAvail()

			ah.lg.Debugw("availability for tower", "tower", tower.Remote, "available", numFree)

			arChan <- availabilityReport{
				tower:   tower,
				layout:  available,
				numFree: numFree,
			}
		}(tower)
	}

	wg.Wait()
	close(arChan)
	<-doneChan

	if len(reports) == 0 {
		ah.lg.Info("no available fixtures")
		return permErrorIfViolated(ah.timesTried, ah.timesToTry, errors.New("no fixtures available"))
	}

	sort.Slice(reports, func(i, j int) bool { return reports[i].numFree > reports[j].numFree })

	if reports[0].numFree == 0 {
		ah.lg.Info("no fixtures in aisle")
		ah.lg.Debugw("reports[0] fixture availability", "report", fmt.Sprintf("%#v", reports[0]))
		ah.lg.Debug(reports[0].layout)

		return permErrorIfViolated(ah.timesTried, ah.timesToTry, errors.New("no fixtures in aisle"))
	}

	ah.first = reports[0]

	ah.lg.Infow("checking if we need another tower", "ah.first.numFree", ah.first.numFree, "len(ah.trays)", len(ah.trays))

	needAnotherTower := ah.first.numFree < 2 && len(ah.trays) == 2

	if !needAnotherTower {
		// we have enough fixtures in this tower for the two trays
		ah.lg.Info("enough fixtures in tower for two trays")
		return nil
	}

	ah.lg.Infow("need another tower", "len_reports", len(reports), "reports", reports)

	// we need a second tower because there wasn't enough room in the first one
	if len(reports) < 2 || reports[1].numFree == 0 {
		// we need a second tower, but none was found with any availability
		ah.lg.Info("not enough fixtures in aisle for two trays")

		return permErrorIfViolated(ah.timesTried, ah.timesToTry, errors.New("not enough fixtures in aisle for two trays"))
	}

	ah.second = reports[1]

	// now check if the second report was for the prohibited column for the back fork
	if ah.second.layout.GetOneFXRForBackFork() == nil {
		// it was, so swap them so the front fork gets that column and the back fork gets the safe one
		ah.lg.Info("second fork was going to get prohibited column for second fork, swapping assignments")
		ah.first, ah.second = ah.second, ah.first
	}

	ah.lg.Info("fixtures in aisle found for two trays")

	return nil
}

func getLocation(aisle *Aisle, lg *zap.SugaredLogger, trays []string, timesToTry int, getter layoutGetter) (operation, error) {
	ah := availabilityHandler{
		timesToTry: timesToTry,
		trays:      trays,
		aisle:      aisle,
		getter:     getter,
		lg:         lg,
	}

	// if the aisle has no availability start to back off until we delay 30 seconds per try
	bo := backoff.NewExponentialBackOff()
	bo.MaxInterval = time.Second * 30

	if err := backoff.Retry(ah.getAvailability, bo); err != nil {
		lg.Errorw("unable to get enough availability for tray(s)", "error", err)
		return operation{}, fmt.Errorf("get aisle availability: %v", err)
	}

	lg.Info("aisle has enough fixtures for tray(s)")

	var (
		front, back availabilityReport
		sendTwo     bool
	)

	switch len(trays) {
	case 1:
		front = ah.first
		front.fixture = ah.first.layout.GetOneFXR()
	case 2:
		if ah.first.numFree == 1 {
			front, back = ah.first, ah.second
			front.fixture, back.fixture = ah.first.layout.GetOneFXR(), ah.second.layout.GetOneFXRForBackFork()
			sendTwo = true
		} else {
			front, back = ah.first, ah.first
			front.fixture, back.fixture = ah.first.layout.GetTwoFXRs()
			if !front.fixture.IsNeighborOf(back.fixture) {
				sendTwo = true
			}
		}

		if front.fixture == nil || back.fixture == nil {
			lg.Errorw("internal error, did not identify two fixtures", "front", front.fixture, "back", back.fixture, "front_layout", ah.first.layout, "back_layout", ah.second.layout)
			return operation{}, fmt.Errorf("internal error, did not identify two fixtures: %#v %#v", front.fixture, back.fixture)
		}

		if back.fixture.Coord.Col == _prohibitedColForBackFork {
			lg.Errorw("internal error, tried to place back tray in prohibited column", "front", front.fixture, "back", back.fixture, "front_layout", ah.first.layout, "back_layout", ah.second.layout)
			return operation{}, fmt.Errorf("internal error, tried to place back tray in prohibited column; fixture: %#v", back.fixture)
		}

	default:
		return operation{}, fmt.Errorf("unexpected number of trays received: %d", len(trays))
	}

	op := operation{
		front:   front,
		back:    back,
		sendTwo: sendTwo,
	}

	lg.Infow("handling operation for tray", "front", op.front.fixture, "back", op.back.fixture, "send_two", op.sendTwo)

	return op, nil
}

func handleIncomingUnload(g asrsapi.Terminal_UnloadOperationsServer, uo *asrsapi.UnloadOperation) error {
	uo.State = &asrsapi.UnloadOperationStateAndStatus{
		State:     asrsapi.UnloadOperationState_PreparedToUnload,
		StateType: asrsapi.StateType_Current,
		Status: &asrsapi.Status{
			Status: asrsapi.Status_Complete,
		},
	}

	return backoff.Retry(func() error {
		return g.Send(uo)
	}, backoff.NewExponentialBackOff())
}

func replaceWorkstationSubID(equipment string, ws, subID string) (string, error) {
	fields := strings.Split(equipment, "-")
	if len(fields) != 4 {
		return "", errors.New("invalid equipment string: " + equipment)
	}

	fields[len(fields)-1] = subID
	fields[len(fields)-2] = ws

	return strings.Join(fields, "-"), nil
}

func replaceAisle(equipment string, aisle string) (string, error) {
	fields := strings.Split(equipment, "-")
	if len(fields) != 4 {
		return "", errors.New("invalid equipment string: " + equipment)
	}

	formAisle := fields[1]
	if len(formAisle) != 5 {
		return "", errors.New("invalid formation aisle string: " + formAisle)
	}

	fields[1] = fmt.Sprintf("%s%s", fields[1][:2], aisle)

	return strings.Join(fields, "-"), nil
}
