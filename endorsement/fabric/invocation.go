/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fabric

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric-x-common/protoutil"
	sdk "github.com/hyperledger/fabric-x-sdk"
	"github.com/hyperledger/fabric-x-sdk/endorsement"
)

var _ endorsement.InvocationBuilder = InvocationBuilder{}

// nonceSize matches the Fabric-X builder, so transaction ids are computed
// over the same input width on both paths.
const nonceSize = 24

// NewInvocationBuilder returns an InvocationBuilder that produces a full
// chaincode proposal. The peer needs the proposal, and the hash is part of
// the endorsement.
func NewInvocationBuilder(signer sdk.Signer) InvocationBuilder {
	return InvocationBuilder{signer: signer}
}

// InvocationBuilder creates Fabric-format invocations from a signer.
type InvocationBuilder struct {
	signer sdk.Signer
}

// NewInvocation creates an Invocation from channel, namespace, chaincode
// version and args. nsVersion must match the namespace's approved chaincode
// version, or the peer rejects the resulting proposal as INVALID_CHAINCODE.
func (b InvocationBuilder) NewInvocation(channel, namespace, nsVersion string, args [][]byte) (endorsement.Invocation, error) {
	if b.signer == nil {
		return endorsement.Invocation{}, errors.New("nil signer")
	}

	creator, err := b.signer.Serialize()
	if err != nil {
		return endorsement.Invocation{}, err
	}

	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return endorsement.Invocation{}, fmt.Errorf("read nonce: %w", err)
	}

	txID := protoutil.ComputeTxID(nonce, creator)
	ccid := &peer.ChaincodeID{Name: namespace, Version: nsVersion}
	proposal, _, err := protoutil.CreateChaincodeProposalWithTxIDNonceAndTransient(
		txID,
		common.HeaderType_ENDORSER_TRANSACTION,
		channel,
		&peer.ChaincodeInvocationSpec{
			ChaincodeSpec: &peer.ChaincodeSpec{
				Type:        peer.ChaincodeSpec_CAR,
				ChaincodeId: ccid,
				Input:       &peer.ChaincodeInput{Args: args},
			},
		},
		nonce,
		creator,
		nil,
	)
	if err != nil {
		return endorsement.Invocation{}, err
	}

	hdr, err := protoutil.UnmarshalHeader(proposal.Header)
	if err != nil {
		return endorsement.Invocation{}, err
	}
	propHash, err := protoutil.GetProposalHash1(hdr, proposal.Payload)
	if err != nil {
		return endorsement.Invocation{}, err
	}

	return endorsement.Invocation{
		TxID:         txID,
		Nonce:        nonce,
		Creator:      creator,
		Args:         args,
		CCID:         ccid,
		Channel:      channel,
		Proposal:     proposal,
		ProposalHash: propHash,
	}, nil
}
