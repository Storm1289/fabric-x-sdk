/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// package endorsement can read a Fabric-style SignedProposal (the request made to a
// peer to execute chaincode). It can also create a signed response, with a transaction
// or read/write set style that can be either Fabric- or Fabric-X format.
//
// Fabric-X does not support "traditional" chaincode, but this package makes it possible to
// create signed endorsements of read/write sets following the new programming model.
package endorsement

import (
	"errors"
	"net/http"

	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric-x-sdk/blocks"
)

// Builder creates the signed ProposalResponse.
type Builder interface {
	Endorse(in Invocation, res ExecutionResult) (*peer.ProposalResponse, error)
}

// InvocationBuilder creates an Invocation. Implementations live next to the
// EndorsementBuilder in each backend; their constructors take a signer.
type InvocationBuilder interface {
	NewInvocation(channel, namespace, chaincodeVersion string, nsVersion uint64, args [][]byte) (Invocation, error)
}

// Invocation instructs the endorser to execute a transaction.
type Invocation struct {
	TxID    string
	Nonce   []byte
	Creator []byte
	Args    [][]byte
	// Namespace is the chaincode/namespace name, read on both protocols.
	Namespace string
	// ChaincodeVersion is Fabric's chaincode-version convention. Unread on Fabric-X.
	ChaincodeVersion string
	// NsVersion is Fabric-X's MVCC namespace-version counter. Unread on Fabric.
	NsVersion uint64
	Channel   string
	Proposal  *peer.Proposal
	// ProposalHash is necessary for the endorsement in Fabric 2 and 3
	ProposalHash []byte
}

// ExecutionResult is the outcome of an execution.
type ExecutionResult struct {
	// RWS is the set of reads and writes as a result of the execution.
	RWS blocks.ReadWriteSet
	// Event is an optional opaque payload that was emitted as a chaincode event.
	Event []byte
	// EventName is the name of the event. It is required when Event is set.
	EventName string
	// Status is a code that should follow the HTTP status codes.
	Status int32
	//Message associated with the response code.
	Message string
	// Payload that can be used to include metadata with this response.
	Payload []byte
}

// ErrMissingEventName is returned when an ExecutionResult has an Event but no EventName.
var ErrMissingEventName = errors.New("event name is required when an event is set")

// Validate reports whether the ExecutionResult can be endorsed.
func (res ExecutionResult) Validate() error {
	if len(res.Event) > 0 && res.EventName == "" {
		return ErrMissingEventName
	}
	return nil
}

func (res ExecutionResult) Response() *peer.Response {
	return &peer.Response{
		Status:  res.Status,
		Message: res.Message,
		Payload: res.Payload,
	}
}

// BadRequest returns a 400 ExecutionResult. Use it when the endorser rejects the invocation
// due to invalid input.
func BadRequest(msg string) ExecutionResult {
	return ExecutionResult{
		Status:  http.StatusBadRequest,
		Message: http.StatusText(http.StatusBadRequest),
		Payload: []byte(msg),
	}
}

// Success returns a 200 ExecutionResult with the given read-write set, event, and response payload.
// eventName is required when event is set.
func Success(rws blocks.ReadWriteSet, eventName string, event []byte, payload []byte) ExecutionResult {
	return ExecutionResult{
		RWS:       rws,
		Event:     event,
		EventName: eventName,
		Status:    200,
		Message:   "OK",
		Payload:   payload,
	}
}
