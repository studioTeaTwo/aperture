package nostr

import (
	"context"
	"fmt"
	"strconv"

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
	UserNPubkey   string           `json:"nPubkey"`
	UserRelayList []string         `json:"relayList,omitempty"`
	Slug          string           `json:"slug"`               // the identifier of blog artcle
	Price         int64            `json:"price"`              // invoice's price, which may be optional using BOLT12 or LNURL in the future
	Preimage      lntypes.Preimage `json:"preimage,omitempty"` // invoice's preimage, which will basically be filled in when the invoice settled
}

func NewNostrClient(serviceNSeckey string, serviceNPubkey string, servicename string, serviceRelayList []string) (*NostrClient, error) {
	_, v, err := nip19.Decode(serviceNSeckey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode service's nSecKey: %w", err)
	}
	seckey := v.(string)

	_, v, err = nip19.Decode(serviceNPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode service's nPubKey: %w", err)
	}
	pubkey := v.(string)

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
	// encrypt the preimage
	// note: message is expected to be the user's receipt
	message := n.servicename + " slug=" + p.Slug + " price=" + strconv.FormatInt(p.Price, 10) + " preimage=" + p.Preimage.String()
	sharedsecret, err := nip04.ComputeSharedSecret(p.UserNPubkey, n.seckey)
	if err != nil {
		return fmt.Errorf("failed to compute shared secret: %w", err)
	}
	msg, err := nip04.Encrypt(message, sharedsecret)
	if err != nil {
		return fmt.Errorf("failed to encrypt preimage: %w", err)
	}

	// create event
	ev := nostr.Event{
		PubKey:    n.nPubkey,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindEncryptedDirectMessage,
		Tags:      nil,
		Content:   msg,
	}
	ev.Tags.AppendUnique(nostr.Tag{"p", p.UserNPubkey})
	ev.Tags.AppendUnique(nostr.Tag{"l402", n.servicename})

	// calling Sign sets the event ID field and the event Sig field
	if err := ev.Sign(n.seckey); err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}

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

		fmt.Printf("published to %s\n", url)
	}
	return nil
}

func DeriveFrom(nSeckey string) (nPubkey string, seckey string, pubkey string, err error) {
	_, s, err := nip19.Decode(nSeckey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to decode seckey: %w", err)
	}
	seckey = s.(string)

	pubkey, err = nostr.GetPublicKey(seckey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get pubkey: %w", err)
	}

	nPubkey, err = nip19.EncodePublicKey(seckey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to encode nPubkey: %w", err)
	}

	return
}
