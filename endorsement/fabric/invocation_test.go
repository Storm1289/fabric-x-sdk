/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fabric

import (
	"bytes"
	"errors"
	"testing"
	"time"

	commonpb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-sdk/endorsement"
)

func newInvocation(t *testing.T) endorsement.Invocation {
	t.Helper()
	inv, err := NewInvocationBuilder(fixedSigner{}).NewInvocation("mychannel", "myns", "v1", [][]byte{[]byte("fn"), []byte("arg")})
	if err != nil {
		t.Fatalf("NewInvocation failed: %v", err)
	}
	return inv
}

func TestNewInvocation_CarriesFullProposal(t *testing.T) {
	inv := newInvocation(t)
	if len(inv.Proposal.Payload) == 0 {
		t.Error("expected a proposal payload; the Fabric peer reads it")
	}
	if len(inv.ProposalHash) == 0 {
		t.Error("expected a proposal hash; it is part of the Fabric endorsement")
	}

	hdr, err := protoutil.UnmarshalHeader(inv.Proposal.Header)
	if err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	chdr, err := protoutil.UnmarshalChannelHeader(hdr.ChannelHeader)
	if err != nil {
		t.Fatalf("unmarshal channel header: %v", err)
	}
	if len(chdr.Extension) == 0 {
		t.Error("expected a chaincode header extension")
	}
	if commonpb.HeaderType(chdr.Type) != commonpb.HeaderType_ENDORSER_TRANSACTION {
		t.Errorf("unexpected header type: %s", commonpb.HeaderType(chdr.Type))
	}
	if chdr.ChannelId != "mychannel" {
		t.Errorf("unexpected channel id: %q", chdr.ChannelId)
	}
}

func TestNewInvocation_TxIDDerivedFromNonceAndCreator(t *testing.T) {
	inv := newInvocation(t)
	if len(inv.Nonce) != nonceSize {
		t.Errorf("expected a %d byte nonce, got %d", nonceSize, len(inv.Nonce))
	}
	if want := protoutil.ComputeTxID(inv.Nonce, inv.Creator); inv.TxID != want {
		t.Errorf("tx id not derived from nonce and creator: got %q, want %q", inv.TxID, want)
	}
	if !bytes.Equal(inv.Creator, []byte("identity")) {
		t.Errorf("unexpected creator: %q", inv.Creator)
	}
}

func TestNewInvocation_NonceIsFresh(t *testing.T) {
	first, second := newInvocation(t), newInvocation(t)
	if bytes.Equal(first.Nonce, second.Nonce) {
		t.Error("two invocations share a nonce")
	}
	if first.TxID == second.TxID {
		t.Error("two invocations share a tx id")
	}
}

func TestNewInvocation_CarriesNamespaceAndArgs(t *testing.T) {
	inv := newInvocation(t)
	if inv.CCID == nil || inv.CCID.Name != "myns" || inv.CCID.Version != "v1" {
		t.Errorf("unexpected chaincode id: %+v", inv.CCID)
	}
	if inv.Channel != "mychannel" {
		t.Errorf("unexpected channel: %q", inv.Channel)
	}
	if len(inv.Args) != 2 || string(inv.Args[0]) != "fn" || string(inv.Args[1]) != "arg" {
		t.Errorf("args do not round-trip: %q", inv.Args)
	}
}

type failingSigner struct{}

func (failingSigner) Sign(_ []byte) ([]byte, error) { return nil, errors.New("sign") }
func (failingSigner) Serialize() ([]byte, error)    { return nil, errors.New("no identity") }

func TestNewInvocation_SerializeError(t *testing.T) {
	_, err := NewInvocationBuilder(failingSigner{}).NewInvocation("mychannel", "myns", "v1", nil)
	if err == nil {
		t.Fatal("expected an error when the signer cannot serialize")
	}
	if err.Error() != "no identity" {
		t.Errorf("expected the signer error to be returned unwrapped, got %v", err)
	}
}

func TestNewInvocation_ParseableByEndorser(t *testing.T) {
	inv := newInvocation(t)
	signed, err := protoutil.GetSignedProposal(inv.Proposal, fixedSigner{})
	if err != nil {
		t.Fatalf("GetSignedProposal: %v", err)
	}
	parsed, err := endorsement.Parse(signed, time.Now())
	if err != nil {
		t.Fatalf("Parse rejected a full proposal: %v", err)
	}
	if parsed.TxID != inv.TxID {
		t.Errorf("parsed tx id %q does not match %q", parsed.TxID, inv.TxID)
	}
	if parsed.Channel != inv.Channel {
		t.Errorf("parsed channel %q does not match %q", parsed.Channel, inv.Channel)
	}
}
