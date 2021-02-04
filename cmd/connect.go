//+build !test

package main

import (
	"net/url"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
	"stash.teslamotors.com/rr/protostream"
)

func connectToProtostreamSocket(sugar *zap.SugaredLogger, host, node string) *protostream.Socket {
	u := url.URL{Scheme: "ws", Host: host, Path: protostream.WSEndpoint}

	var (
		sub *protostream.Socket
		err error
	)

	if err = backoff.Retry(
		func() error {
			sub, err = protostream.NewSubscriberWithSub(u.String(), node)
			if err != nil {
				sugar.Errorw("create new subscriber", "error", err)
				return err
			}

			return nil
		},
		backoff.NewExponentialBackOff(), // defaults are fine on startup
	); err != nil {
		sugar.Fatalw("create new subscriber", "error", err)
	}

	return sub
}
