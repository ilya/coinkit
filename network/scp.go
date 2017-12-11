package network

import (
)

// Stuff for implementing the Stellar Consensus Protocol. See:
// https://www.stellar.org/papers/stellar-consensus-protocol.pdf
// When there are frustrating single-letter variable names, it's because we are
// making the names line up with the protocol paper.

// For now each block just has a list of comments.
// This isn't supposed to be useful, it's just for testing.
type SlotValue struct {
	Comments []string
}

type QuorumSlice struct {
	// Members is a list of public keys for nodes that occur in the quorum slice.
	// Typically includes ourselves.
	Members []string

	// The number of members we require for consensus, including ourselves.
	// The protocol can support other sorts of slices, like weighted or any wacky
	// thing, but for now we only do this simple "any k out of these n" voting.
	Threshold int
}

type NominateMessage struct {
	// What slot we are nominating values for
	I int

	// The values we have voted to nominate
	X []SlotValue

	// The values we have accepted as nominated
	Y []SlotValue
	
	D QuorumSlice
}

func (m *NominateMessage) MessageType() string {
	return "Nominate"
}

// See page 21 of the protocol paper for more detail here.
type NominationState struct {
	// The values we have voted to nominate
	X []SlotValue

	// The values we have accepted as nominated
	Y []SlotValue

	// The values that we consider to be candidates 
	Z []SlotValue

	// The last NominateMessage received from each node
	N map[string]*NominateMessage
}

// Ballot phases
type Phase int
const (
	Prepare Phase = iota
	Confirm
	Externalize
)

type Ballot struct {
	// An increasing counter, n >= 1, to ensure we can always have more ballots
	n int

	// The value this ballot proposes
	x SlotValue
}

// See page 23 of the protocol paper for more detail here.
type BallotState struct {
	// The current ballot we are trying to prepare and commit.
	b *Ballot

	// The highest two ballots that are accepted as prepared.
	// p is the highest, pPrime the next.
	// It's nil if there is no such ballot.
	p *Ballot
	pPrime *Ballot

	// In the Prepare phase, c is the lowest and h is the highest ballot
	// for which we have voted to commit but not accepted abort.
	// In the Confirm phase, c is the lowest and h is the highest ballot
	// for which we accepted commit.
	// In the Externalize phase, c is the lowest and h is the highest ballot
	// for which we confirmed commit.
	// If c is not nil, then c <= h <= b.
	c *Ballot
	h *Ballot

	// The value to use in the next ballot
	z SlotValue
	
	// The latest PrepareMessage, ConfirmMessage, or ExternalizeMessage from each peer
	M map[string]Message
}

// PrepareMessage is the first phase of the three-phase ballot protocol
type PrepareMessage struct {
	// TODO
}

func (m *PrepareMessage) MessageType() string {
	return "Prepare"
}

// ConfirmMessage is the second phase of the three-phase ballot protocol
type ConfirmMessage struct {
	// TODO
}

func (m *ConfirmMessage) MessageType() string {
	return "Confirm"
}

// ExternalizeMessage is the third phase of the three-phase ballot protocol
type ExternalizeMessage struct {
	// TODO
}

func (m *ExternalizeMessage) MessageType() string {
	return "Externalize"
}

type StateBuilder struct {
	// Which slot is actively being built
	slot int

	// Values for past slots that have already achieved consensus
	map[int]SlotValue values

	nState NominationState
	bState BallotState
}

func NewStateBuilder() *StateBuilder {
	// TODO,
}
