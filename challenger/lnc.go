package challenger

import (
	"fmt"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/studioTeaTwo/aperture/lnc"
)

// LNCChallenger is a challenger that uses LNC to connect to an lnd backend to
// create new LSAT payment challenges.
type LNCChallenger struct {
	lndChallenger *LndChallenger
	nodeConn      *lnc.NodeConn
}

// NewLNCChallenger creates a new challenger that uses the given LNC session to
// connect to an lnd backend to create payment challenges.
func NewLNCChallenger(session *lnc.Session, lncStore lnc.Store,
	genInvoiceReq InvoiceRequestGenerator,
	errChan chan<- error) (*LNCChallenger, error) {

	nodeConn, err := lnc.NewNodeConn(session, lncStore)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to lnd using lnc: %w",
			err)
	}

	client, err := nodeConn.Client()
	if err != nil {
		return nil, err
	}

	lndChallenger, err := NewLndChallenger(
		client, genInvoiceReq, nodeConn.CtxFunc, errChan,
	)
	if err != nil {
		return nil, err
	}

	err = lndChallenger.Start()
	if err != nil {
		return nil, err
	}

	return &LNCChallenger{
		lndChallenger: lndChallenger,
		nodeConn:      nodeConn,
	}, nil
}

// Stop stops the challenger.
func (l *LNCChallenger) Stop() {
	err := l.nodeConn.Stop()
	if err != nil {
		log.Errorf("unable to stop lnc node conn: %v", err)
	}

	l.lndChallenger.Stop()
}

// NewChallenge creates a new LSAT payment challenge, returning a payment
// request (invoice) and the corresponding payment hash.
//
// NOTE: This is part of the mint.Challenger interface.
func (l *LNCChallenger) NewChallenge(price int64, memo MemoParam) (string, lntypes.Hash,
	error) {

	return l.lndChallenger.NewChallenge(price, memo)
}

// VerifyInvoiceStatus checks that an invoice identified by a payment
// hash has the desired status. To make sure we don't fail while the
// invoice update is still on its way, we try several times until either
// the desired status is set or the given timeout is reached.
//
// NOTE: This is part of the auth.InvoiceChecker interface.
func (l *LNCChallenger) VerifyInvoiceStatus(hash lntypes.Hash,
	state lnrpc.Invoice_InvoiceState, timeout time.Duration) error {

	return l.lndChallenger.VerifyInvoiceStatus(hash, state, timeout)
}
