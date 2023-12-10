package nostr

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip19"
)

const ()

var (
	MockNostrClient = &NostrClient{}
)

type NostrClient struct {
	nSeckey     string // service's account
	nPubkey     string // service's account
	seckey      string // service's account
	pubkey      string // service's account
	relayList   []string
	servicename string // l402 service name
}

type NostrPublishParam struct {
	UserNPubkey   string   `json:"nPubkey"`
	UserRelayList []string `json:"relayList,omitempty"`
	Slug          string   `json:"slug"`  // the identifier of blog artcle
	Price         int64    `json:"price"` // invoice's price, which may be optional using BOLT12 or LNURL in the future
	Invoice       *lnrpc.Invoice
}

func NewNostrClient(serviceNSeckey string, servicename string, serviceRelayList []string) (*NostrClient, error) {
	_, v, err := nip19.Decode(serviceNSeckey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode service's nSecKey: %w", err)
	}
	seckey := v.(string)

	pubkey, err := nostr.GetPublicKey(seckey)
	if err != nil {
		return nil, fmt.Errorf("failed to get service's pubkey: %w", err)
	}

	serviceNPubkey, err := nip19.EncodePublicKey(pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode service's nPubKey: %w", err)
	}

	return &NostrClient{
		nSeckey:     serviceNSeckey,
		nPubkey:     serviceNPubkey,
		seckey:      seckey,
		pubkey:      pubkey,
		relayList:   serviceRelayList,
		servicename: servicename,
	}, nil
}

func (n *NostrClient) PublishEvent(p *NostrPublishParam) error {
	_, v, err := nip19.Decode(p.UserNPubkey)
	if err != nil {
		return fmt.Errorf("failed to decode user's nPubKey: %w", err)
	}
	userPubkey := v.(string)
	preimage, err := lntypes.MakePreimage(p.Invoice.GetRPreimage())
	if err != nil {
		return fmt.Errorf("error making invoice preimage: %w", err)
	}
	paymentHash, err := lntypes.MakeHash(p.Invoice.GetRHash())
	if err != nil {
		return fmt.Errorf("error making invoice payment hash: %w", err)
	}

	// encrypt the preimage
	// note: message is expected to be the user's receipt
	message := n.servicename +
		" article=" + p.Slug +
		" settleDate=" + time.Unix(p.Invoice.GetSettleDate(), 0).Format("2006-01-02T15:04:05") +
		" price=" + strconv.FormatInt(p.Price*1000, 10) +
		" paidAmount=" + strconv.FormatInt(p.Invoice.GetAmtPaidMsat(), 10) +
		" preimage=" + preimage.String() +
		" paymentHash=" + paymentHash.String()
	sharedsecret, err := nip04.ComputeSharedSecret(userPubkey, n.seckey)
	if err != nil {
		return fmt.Errorf("failed to compute shared secret: %w", err)
	}
	msg, err := nip04.Encrypt(message, sharedsecret)
	if err != nil {
		return fmt.Errorf("failed to encrypt preimage: %w", err)
	}

	// create event
	ev := nostr.Event{
		PubKey:    n.pubkey,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindEncryptedDirectMessage,
		Tags:      nil,
		Content:   msg,
	}
	ev.Tags = nostr.Tags{}
	ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"p", userPubkey})
	ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"L", "#l402"})
	ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"l", n.servicename, "#l402"})
	relays := nostr.Tag{"relays"}
	for _, v := range n.relayList {
		relays = append(relays, v)
	}
	ev.Tags = ev.Tags.AppendUnique(relays)

	// calling Sign sets the event ID field and the event Sig field
	if err := ev.Sign(n.seckey); err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}

	log.Infof("prepared new Nostr's event: %+v", ev)

	// publish the event to two relays
	ctx := context.Background()
	// TODO: also publish the relay which the user subscribes
	for _, url := range n.relayList {
		relay, err := nostr.RelayConnect(ctx, url)
		if err != nil {
			fmt.Println(err)
			continue
		}
		_, err = relay.Publish(ctx, ev)
		if err != nil {
			fmt.Println(err)
			continue
		}

		log.Infof("published to %s event:%s", url, ev.ID)
	}
	return nil
}
