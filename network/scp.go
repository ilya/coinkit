package network

import (
	"log"
	"strings"
	"time"
)

// Stuff for implementing the Stellar Consensus Protocol. See:
// https://www.stellar.org/papers/stellar-consensus-protocol.pdf
// When there are frustrating single-letter variable names, it's because we are
// making the names line up with the protocol paper.

type NominationMessage struct {
	// What slot we are nominating values for
	I int

	// The values we have voted to nominate
	Nom []SlotValue

	// The values we have accepted as nominated
	Acc []SlotValue
	
	D QuorumSlice
}

func (m *NominationMessage) MessageType() string {
	return "Nomination"
}

// See page 21 of the protocol paper for more detail here.
type NominationState struct {
	// The values we have voted to nominate
	X []SlotValue

	// The values we have accepted as nominated
	Y []SlotValue

	// The values whose nomination we have confirmed
	Z []SlotValue

	// The last NominationMessage received from each node
	N map[string]*NominationMessage

	// Who we are
	publicKey string

	// Who we listen to for quorum
	D QuorumSlice

	// The number of non-duplicate messages this state has processed
	received int
}

func NewNominationState(publicKey string, qs QuorumSlice) *NominationState {
	return &NominationState{
		X: make([]SlotValue, 0),
		Y: make([]SlotValue, 0),
		Z: make([]SlotValue, 0),
		N: make(map[string]*NominationMessage),
		publicKey: publicKey,
		D: qs,
	}
}

func (s *NominationState) Logf(format string, a ...interface{}) {
	// log.Printf(format, a...)
}

func (s *NominationState) Show() {
	s.Logf("nState for %s:", s.publicKey)
	s.Logf("X: %+v", s.X)
	s.Logf("Y: %+v", s.Y)
	s.Logf("Z: %+v", s.Z)
}

// HasNomination tells you whether this nomination state can currently send out
// a nominate message.
// If we have never received a nomination from a peer, and haven't had SetDefault
// called ourselves, then we won't have a nomination.
func (s *NominationState) HasNomination() bool {
	return len(s.X) > 0
}

func (s *NominationState) SetDefault(v SlotValue) {
	if s.HasNomination() {
		// We already have something to nominate
		return
	}
	s.X = []SlotValue{v}
}

// PredictValue can predict the value iff HasNomination is true. If not, panic
func (s *NominationState) PredictValue() SlotValue {
	if len(s.Z) > 0 {
		return CombineSlice(s.Z)
	}
	if len(s.Y) > 0 {
		answer := CombineSlice(s.Y)
		return answer
	}
	if len(s.X) > 0 {
		return CombineSlice(s.X)
	}
	panic("PredictValue was called when HasNomination was false")
}

func (s *NominationState) QuorumSlice(node string) (*QuorumSlice, bool) {
	if node == s.publicKey {
		return &s.D, true
	}
	m, ok := s.N[node]
	if !ok {
		return nil, false
	}
	return &m.D, true
}

func (s *NominationState) PublicKey() string {
	return s.publicKey
}

func (s *NominationState) AssertValid() {
	AssertNoDupes(s.X)
	AssertNoDupes(s.Y)
	AssertNoDupes(s.Z)
}

// MaybeAdvance checks whether we should accept the nomination for this slot value,
// and adds it to our accepted list if appropriate.
// It also checks whether we should confirm the nomination.
// Returns whether we made any changes.
func (s *NominationState) MaybeAdvance(v SlotValue) bool {
	if HasSlotValue(s.Z, v) {
		// We already confirmed this, so we can't do anything more
		return false
	}
	
	changed := false	
	votedOrAccepted := []string{}
	accepted := []string{}
	if HasSlotValue(s.X, v) {
		votedOrAccepted = append(votedOrAccepted, s.publicKey)
	}
	if HasSlotValue(s.Y, v) {
		accepted = append(accepted, s.publicKey)
	}
	for node, m := range s.N {
		if HasSlotValue(m.Acc, v) {
			votedOrAccepted = append(votedOrAccepted, node)
			accepted = append(accepted, node)
			continue
		}
		if HasSlotValue(m.Nom, v) {
			votedOrAccepted = append(votedOrAccepted, node)
		}
	}

	// The rules for accepting are on page 13, section 5.3
	// Rule 1: if a quorum has either voted for the nomination or accepted the
	// nomination, we accept it.
	// Rule 2: if a blocking set for us accepts the nomination, we accept it.
	accept := MeetsQuorum(s, votedOrAccepted) || s.D.BlockedBy(accepted)

	if accept && !HasSlotValue(s.Y, v) {
		// Accept this value
		s.Logf("%s accepts the nomination of %+v", s.publicKey, v)
		changed = true
		s.Logf("old s.Y: %+v", s.Y)		
		AssertNoDupes(s.Y)
		s.Y = append(s.Y, v)
		accepted = append(accepted, s.publicKey)
		s.Logf("new s.Y: %+v", s.Y)
		AssertNoDupes(s.Y)
	}

	// We confirm once a quorum has accepted
	if MeetsQuorum(s, accepted) {
		s.Logf("%s confirms the nomination of %+v", s.publicKey, v)		
		changed = true
		s.Z = append(s.Z, v)
		s.Logf("new s.Z: %+v", s.Z)
	}
	return changed
}

// Handles an incoming nomination message from a peer node
func (s *NominationState) Handle(node string, m *NominationMessage) {
	// What nodes we have seen new information about
	touched := []SlotValue{}

	// Check if there's anything new
	old, ok := s.N[node]
	var oldLenNom, oldLenAcc int
	if ok {
		oldLenNom = len(old.Nom)
		oldLenAcc = len(old.Acc)
	}
	if len(m.Nom) < oldLenNom {
		s.Logf("%s sent a stale message: %v", node, m)
		return
	}
	if len(m.Acc) < oldLenAcc {
		s.Logf("%s sent a stale message: %v", node, m)
		return
	}
	if len(m.Nom) == oldLenNom && len(m.Acc) == oldLenAcc {
		// It's just a dupe
		return
	}
	// Update our most-recent-message
	s.Logf("\n\n%s got nomination message from %s:\n%+v", s.publicKey, node, m)
	s.N[node] = m
	s.received++
	
	for i := oldLenNom; i < len(m.Nom); i++ {
		if !HasSlotValue(touched, m.Nom[i]) {
			touched = append(touched, m.Nom[i])
		}

		// If we don't have a candidate, we can support this new nomination
		if !HasSlotValue(s.X, m.Nom[i]) {
			s.Logf("%s supports the nomination of %+v", s.publicKey, m.Nom[i])
			s.X = append(s.X, m.Nom[i])
			s.Logf("new s.X: %+v", s.X)
		}
	}

	for i := oldLenAcc; i < len(m.Acc); i++ {
		if !HasSlotValue(touched, m.Acc[i]) {
			touched = append(touched, m.Acc[i])
		}
	}
	for _, v := range touched {
		s.MaybeAdvance(v)
	}
}

// See page 23 of the protocol paper for more detail here.
// The null ballot is represented by a nil.
type BallotState struct {
	// What phase of balloting we are in
	phase Phase
	
	// The current ballot we are trying to prepare and commit.
	b *Ballot

	// The last value of b we saw during validation.
	// This is just used to make sure the values of b are monotonically
	// increasing, to ensure we don't vote for contradictory things.
	last *Ballot
	
	// The highest two incompatible ballots that are accepted as prepared.
	// p is the highest, pPrime the next.
	// It's nil if there is no such ballot.
	p *Ballot
	pPrime *Ballot

	// [cn, hn] defines a range of ballot numbers that defines a range of
	// b-compatible ballots.
	// [0, 0] is the invalid range, rather than [0], since 0 is an invalid ballot
	// number.
	// In the Prepare phase, this is the range we have voted to commit (which
	// we do when we can confirm the ballot is prepared) but that we have not
	// aborted.
	// In the Confirm phase, this is the range we have accepted a commit.
	// In the Externalize phase, this is the range we have confirmed a commit.
	cn int
	hn int

	// The value to use in the next ballot, if this ballot fails.
	// We may have no idea what value we would use. In that case, z is nil.
	z *SlotValue
	
	// The latest PrepareMessage, ConfirmMessage, or ExternalizeMessage from
	// each peer
	M map[string]BallotMessage

	// Who we are
	publicKey string

	// Who we listen to for quorum
	D QuorumSlice

	// The number of non-duplicate messages this state has processed
	received int
}

func NewBallotState(publicKey string, qs QuorumSlice) *BallotState {
	return &BallotState{
		phase: Prepare,
		M: make(map[string]BallotMessage),
		publicKey: publicKey,
		D: qs,
	}
}

func (s *BallotState) Logf(format string, a ...interface{}) {
	log.Printf(format, a...)
}

func (s *BallotState) Show() {
	log.Printf("bState for %s:", s.publicKey)
	log.Printf("b: %+v", s.b)
	log.Printf("p: %+v", s.p)
	log.Printf("pPrime: %+v", s.pPrime)
	log.Printf("c: %d", s.cn)
	log.Printf("h: %d", s.hn)
	log.Printf("z: %+v", s.z)
}

func (s *BallotState) PublicKey() string {
	return s.publicKey
}

func (s *BallotState) QuorumSlice(node string) (*QuorumSlice, bool) {
	if node == s.publicKey {
		return &s.D, true
	}
	m, ok := s.M[node]
	if !ok {
		return nil, false
	}
	qs := m.QuorumSlice()
	return &qs, true
}

// MaybeAcceptAsPrepared returns true if the ballot state changes.
func (s *BallotState) MaybeAcceptAsPrepared(n int, x SlotValue) bool {
	if s.phase != Prepare {
		return false
	}
	if n == 0 {
		return false
	}

	// Check if we already accept this as prepared
	if s.p != nil && s.p.n >= n && Equal(s.p.x, x) {
		return false
	}
	if s.pPrime != nil && s.pPrime.n >= n && Equal(s.pPrime.x, x) {
		return false
	}

	if s.pPrime != nil && s.pPrime.n >= n {
		// This is about an old ballot number, we don't care even if it is
		// accepted
		return false
	}
	
	// The rules for accepting are, if a quorum has voted or accepted,
	// we can accept.
	// Or, if a local blocking set has accepted, we can accept.
	votedOrAccepted := []string{}
	accepted := []string{}
	if s.b != nil && s.b.n >= n && Equal(s.b.x, x) {
		// We have voted for this
		votedOrAccepted = append(votedOrAccepted, s.publicKey)
	}

	for node, m := range s.M {
		if m.AcceptAsPrepared(n, x) {
			accepted = append(accepted, node)
			votedOrAccepted = append(votedOrAccepted, node)
			continue
		}
		if m.VoteToPrepare(n, x) {
			votedOrAccepted = append(votedOrAccepted, node)
		}
	}

	if !MeetsQuorum(s, votedOrAccepted) && !s.D.BlockedBy(accepted) {
		// We can't accept this as prepared yet
		return false
	}

	s.Logf("%s accepts as prepared: %d %+v", s.publicKey, n, x)
	ballot := &Ballot{
		n: n,
		x: x,
	}
	
	if s.b != nil && s.b.n <= n && !Equal(s.b.x, x) {
		// Accepting this as prepared means we have to abort b
		// Let's switch our active ballot to this one. It should be okay since
		// we are accepting the abort of b, even though we may have voted
		// for the commit of b.
		s.Logf("%s accepts the abort of %+v", s.publicKey, s.b)
		s.cn = 0
		s.b = ballot
	}
	
	// p and p prime should be the top two conflicting things we accept
	// as prepared. update them accordingly
	if s.p == nil {
		s.p = ballot
	} else if Equal(s.p.x, x) {
		if n <= s.p.n {
			log.Fatal("should have short circuited already")
		}
		s.p = ballot
	} else if n >= s.p.n {
		s.pPrime = s.p
		s.p = ballot
	} else {
		// We already short circuited if it isn't worth bumping p prime
		s.pPrime = ballot
	}

	// Check if accepting this prepare means that we should give up some
	// of our votes to commit
	if s.b != nil {
		for s.cn != 0 && s.AcceptedAbort(s.cn, s.b.x) {
			s.Logf("%s accepts the abort of %d %+v", s.cn, s.b.x)
			s.cn++
			if s.cn > s.hn {
				s.cn = 0
			}
		}
	}

	return true
}

// AcceptedAbort returns whether we have already accepted an abort for the
// ballot number and slot value provided.
func (s *BallotState) AcceptedAbort(n int, x SlotValue) bool {
	if s.phase != Prepare {
		// After the prepare phase, we've accepted an abort for everything
		// else.
		return !Equal(x, s.b.x)
	}

	if s.p != nil && s.p.n >= n && !Equal(s.p.x, x) {
		// we accept p is prepared, which implies we accept this abort
		return true
	}

	if s.pPrime != nil && s.pPrime.n >= n && !Equal(s.pPrime.x, x) {
		// we accept p' is prepared, which implies we accept this abort
		return true
	}

	// No reason to think we accept this abort
	return false
}

// MaybeConfirmAsPrepared returns whether anything in the ballot state changed.
func (s *BallotState) MaybeConfirmAsPrepared(n int, x SlotValue) bool {
	if s.phase != Prepare {
		return false
	}
	if s.hn >= n {
		// We already confirmed a ballot as prepared that is at least
		// as good as this one.
		return false
	}

	ballot := &Ballot{
		n: n,
		x: x,
	}
	
	// We confirm when a quorum accepts as prepared
	accepted := []string{}
	if gtecompat(s.p, ballot) || gtecompat(s.pPrime, ballot) {
		// We accept as prepared
		accepted = append(accepted, s.publicKey)
	}

	for node, m := range s.M {
		if m.AcceptAsPrepared(n, x) {
			accepted = append(accepted, node)
		}
	}

	if !MeetsQuorum(s, accepted) {
		return false
	}

	s.Logf("%s confirms as prepared: %d %+v", s.publicKey, n, x)

	if s.cn > 0 && !Equal(x, s.b.x) {
		s.Show()
		log.Fatalf("we are voting to commit but must confirm a contradiction")
	}

	s.hn = n
	s.z = &x
	
	if s.b == nil {
		// We weren't working on any ballot, but now we can work on this one
		s.b = ballot
	}
	
	if s.cn == 0 && Equal(x, s.b.x) {
		// Check if we should start voting to commit
		if gteincompat(s.p, ballot) || gteincompat(s.pPrime, ballot) {
			// We have already accepted the abort of this. So nope.
		} else if s.b.n > n {
			// We are already past this ballot number. We might have
			// even voted to abort it. So we can't vote to commit.
		} else {
			s.cn = s.b.n
		}
	}
	return true
}

// MaybeAcceptAsCommitted returns whether anything in the ballot state changed.
func (s *BallotState) MaybeAcceptAsCommitted(n int, x SlotValue) bool {
	if s.phase == Externalize {
		return false
	}
	if s.phase == Confirm && s.cn <= n && n <= s.hn {
		// We already do accept this commit
		return false
	}

	votedOrAccepted := []string{}
	accepted := []string{}

	if s.phase == Prepare && s.b != nil &&
		Equal(s.b.x, x) && s.cn != 0 && s.cn <= n && n <= s.hn {
		// We vote to commit this
		votedOrAccepted = append(votedOrAccepted, s.publicKey)
	}

	for node, m := range s.M {
		if m.AcceptAsCommitted(n, x) {
			votedOrAccepted = append(votedOrAccepted, node)
			accepted = append(accepted, node)
		} else if m.VoteToCommit(n, x) {
			votedOrAccepted = append(votedOrAccepted, node)
		}
	}

	if !MeetsQuorum(s, votedOrAccepted) && !s.D.BlockedBy(accepted) {
		// We can't accept this commit yet
		return false
	}

	s.Logf("%s accepts as committed: %d %+v", s.publicKey, n, x)
	
	// We accept this commit
	s.phase = Confirm
	if s.b == nil || !Equal(s.b.x, x) {
		// Totally replace our old target value
		s.b = &Ballot{
			n: n,
			x: x,
		}
		s.cn = n
		s.hn = n
		s.z = &x
	} else {
		// Just update our range of acceptance
		if n < s.cn {
			s.cn = n
		}
		if n > s.hn {
			s.hn = n
		}
	}
	return true
}

// MaybeConfirmAsCommitted returns whether anything in the ballot state changed.
func (s *BallotState) MaybeConfirmAsCommitted(n int, x SlotValue) bool {
	if s.phase == Prepare {
		return false
	}
	if s.b == nil || !Equal(s.b.x, x) {
		return false
	}
	
	accepted := []string{}
	if s.phase == Confirm {
		if s.cn <= n && n <= s.hn {
			accepted = append(accepted, s.publicKey)
		}
	} else if s.cn <= n && n <= s.hn {
		// We already did confirm this as committed
		return false
	}

	for node, m := range s.M {
		if m.AcceptAsCommitted(n, x) {
			accepted = append(accepted, node)
		}
	}

	if !MeetsQuorum(s, accepted) {
		return false
	}
	
	s.Logf("%s confirms as committed: %d %+v", s.publicKey, n, x)
	
	if s.phase == Confirm {
		s.phase = Externalize
		s.cn = n
		s.hn = n
	} else {
		if n < s.cn {
			s.cn = n
		}
		if n > s.hn {
			s.hn = n
		}
	}

	return true
}

// Returns whether we needed to bump the ballot number.
// We bump the ballot number if the set of nodes that could never vote
// for our ballot is blocking.
func (s *BallotState) MaybeNextBallot() bool {
	if s.z == nil || s.b == nil {
		return false
	}

	// Nodes that could never vote for our ballot
	blockers := []string{}

	for node, m := range s.M {
		if !m.CouldEverVoteFor(s.b.n, s.b.x) {
			blockers = append(blockers, node)
		}
	}

	if !s.D.BlockedBy(blockers) {
		return false
	}

	// Use s.z for the next ballot
	b := &Ballot{
		n: s.b.n + 1,
		x: *s.z,
	}
	s.Logf("our old ballot %+v cannot pass, bump up to %+v", s.b, b)
	s.b = b
	return true
}

// Update the stage of this ballot as needed
// See the handling algorithm on page 24 of the Mazieres paper.
// The investigate method does steps 1-8
func (s *BallotState) Investigate(n int, x SlotValue) {
	s.MaybeAcceptAsPrepared(n, x)
	s.MaybeConfirmAsPrepared(n, x)
	s.MaybeAcceptAsCommitted(n, x)
	s.MaybeConfirmAsCommitted(n, x)
}

// SelfInvestigate checks whether the current ballot can be advanced
func (s *BallotState) SelfInvestigate() {
	if s.b == nil {
		return
	}
	s.Investigate(s.b.n, s.b.x)
}

func (s *BallotState) Handle(node string, message BallotMessage) {
	// If this message isn't new, skip it
	old, ok := s.M[node]
	if ok && Compare(old, message) >= 0 {
		return
	}
	s.Logf("\n\n%s got ballot message from %s:\n%+v", s.publicKey, node, message)
	s.received++
	s.M[node] = message

	for {
		// Investigate all ballots whose state might be updated
		// TODO: make sure we aren't missing ballot numbers, either internal to
		// the ranges or because a confirm implies many prepares. Maybe we can
		// make investigation be per-value rather than per-ballot?
		// TODO: make sure a malformed message can't DDOS us here
		switch m := message.(type) {
		case *PrepareMessage:
			s.Investigate(m.Bn, m.Bx)
			s.Investigate(m.Pn, m.Px)
			s.Investigate(m.Ppn, m.Ppx)
		case *ConfirmMessage:
			s.Investigate(m.Hn, m.X)
		case *ExternalizeMessage:
			for i := m.Cn; i <= m.Hn; i++ {
				s.Investigate(i, m.X)
			}
		}

		// Step 9 of the processing algorithm
		if !s.MaybeNextBallot() {
			break
		}
	}
}

// MaybeInitializeValue initializes the value if it doesn't already have a value,
// and returns whether anything in the ballot state changed.
func (s *BallotState) MaybeInitializeValue(v SlotValue) bool {
	if s.z != nil {
		return false
	}
	s.z = &v
	if s.b == nil {
		s.b = &Ballot{
			n: 1,
			x: v,
		}
	}
	return true
}

// MaybeUpdateValue updates the value from the nomination if we are supposed to.
// Returns whether anything in the ballot state changed.
func (s *BallotState) MaybeUpdateValue(ns *NominationState) bool {
	if s.hn != 0 {
		// While we have a confirmed prepared ballot, we don't
		// override it based on nominations.
		return false
	}

	if !ns.HasNomination() {
		// No idea how to set the value
		return false
	}
	v := ns.PredictValue()

	if s.z != nil && Equal(v, *s.z) {
		// The new value is the same as the old one
		return false
	}

	// s.Logf("%s updating value to %+v", s.publicKey, v)
	s.z = &v

	if s.b == nil {
		s.b = &Ballot{
			n: 1,
			x: v,
		}
	} else {
		// TODO: in the paper this part only happens when a timer fires, but that
		// seems to be an optimization so I punted for now.
		// See page 25
		s.b = &Ballot{
			n: s.b.n + 1,
			x: v,
		}
		// s.Logf("new value, bumping the ballot to %+v", s.b)
	}
	
	return true
}

func (s *BallotState) HasMessage() bool {
	return s.b != nil
}

func (s *BallotState) AssertValid() {
	if s.cn > s.hn {
		s.Show()
		log.Fatalf("c should be <= h")
	}
	
	if s.p != nil && s.pPrime != nil && Equal(s.p.x, s.pPrime.x) {
		log.Printf("p: %+v", s.p)
		log.Printf("pPrime: %+v", s.pPrime)
		log.Fatalf("p and p prime should not be compatible")
	}

	if s.b != nil && s.phase == Prepare {
		if s.p != nil && !Equal(s.b.x, s.p.x) && s.cn != 0 && s.cn <= s.p.n {
			log.Printf("b: %+v", s.b)
			log.Printf("c: %d", s.cn)
			log.Printf("p: %+v", s.p)
			log.Fatalf("the vote to commit should have been aborted")
		}
		if s.pPrime != nil && !Equal(s.b.x, s.pPrime.x) && s.cn != 0 && s.cn <= s.pPrime.n {
			log.Printf("b: %+v", s.b)
			log.Printf("c: %d", s.cn)			
			log.Printf("pPrime: %+v", s.pPrime)
			log.Fatalf("the vote to commit should have been aborted")
		}
	}

	if s.b != nil && s.phase == Prepare {
		if s.last != nil && !Equal(s.b.x, s.last.x) && s.last.n > s.b.n {
			log.Printf("last b: %+v", s.last)
			log.Printf("curr b: %+v", s.b)
			log.Fatalf("monotonicity violation")
		}
		
		s.last = s.b
	}
}

func (s *BallotState) Message(slot int, qs QuorumSlice) Message {
	if !s.HasMessage() {
		panic("coding error")
	}

	switch s.phase {
	case Prepare:
		m := &PrepareMessage{
			T: Prepare,
			I: slot,
			Bn: s.b.n,
			Bx: s.b.x,
			Cn: s.cn,
			Hn: s.hn,
			D: qs,
		}
		if s.p != nil {
			m.Pn = s.p.n
			m.Px = s.p.x
		}
		if s.pPrime != nil {
			m.Ppn = s.pPrime.n
			m.Ppx = s.pPrime.x
		}
		return m

	case Confirm:
		m := &ConfirmMessage{
			T: Confirm,
			I: slot,
			X: s.b.x,
			Cn: s.cn,
			Hn: s.hn,
			D: qs,
		}
		if s.p != nil {
			m.Pn = s.p.n
		}
		return m

	case Externalize:
		return &ExternalizeMessage{
			T: Externalize,
			I: slot,
			X: s.b.x,
			Cn: s.cn,
			Hn: s.hn,
			D: qs,
		}
	}

	panic("code flow should not get here")
}

type ChainState struct {
	// Which slot is actively being built
	slot int

	// The time we started working on this slot
	start time.Time
	
	// Values for past slots that have already achieved consensus
	values map[int]SlotValue

	nState *NominationState
	bState *BallotState

	// Who we care about
	D QuorumSlice

	// Who we are
	publicKey string
}

func NewChainState(publicKey string, members []string, threshold int) *ChainState {
	log.Printf("I am %s", publicKey)
	qs := QuorumSlice{
		Members: members,
		Threshold: threshold,
	}
	return &ChainState{
		slot: 1,
		start: time.Now(),
		values: make(map[int]SlotValue),
		nState: NewNominationState(publicKey, qs),
		bState: NewBallotState(publicKey, qs),
		D: qs,
		publicKey: publicKey,
	}
}

func (cs *ChainState) AssertValid() {
	cs.nState.AssertValid()
	cs.bState.AssertValid()
}

// OutgoingMessages returns the outgoing messages.
// There can be zero or one nomination messages, and zero or one ballot messages.
func (cs *ChainState) OutgoingMessages() []Message {
	answer := []Message{}

	if !cs.nState.HasNomination() {
		// There's nothing to nominate. Let's nominate something.
		// TODO: if it's not our turn, wait instead of nominating
		comment := strings.Replace(cs.publicKey, "node", "comment", 1)
		v := MakeSlotValue(comment)
		log.Printf("%s nominates %+v", cs.publicKey, v)
		cs.nState.SetDefault(v)
	}

	answer = append(answer, &NominationMessage{
		I: cs.slot,
		Nom: cs.nState.X,
		Acc: cs.nState.Y,
		D: cs.D,
	})

	// If we aren't working on any ballot, but we do have a nomination, we can
	// optimistically start working on that ballot
	if cs.nState.HasNomination() && cs.bState.z == nil {
		cs.bState.MaybeInitializeValue(cs.nState.PredictValue())
	}
	
	if cs.bState.HasMessage() {
		answer = append(answer, cs.bState.Message(cs.slot, cs.D))
	}

	return answer
}

// Done returns whether this chain has externalized all the slots it is working on.
func (cs *ChainState) Done() bool {
	return cs.bState.phase == Externalize
}

// Handle handles an incoming message
func (cs *ChainState) Handle(sender string, message Message) {
	if sender == cs.publicKey {
		// It's one of our own returning to us, we can ignore it
		return
	}
	switch m := message.(type) {
	case *NominationMessage:
		cs.nState.Handle(sender, m)
		cs.bState.MaybeUpdateValue(cs.nState)
	case *PrepareMessage:
		cs.bState.Handle(sender, m)
	case *ConfirmMessage:
		cs.bState.Handle(sender, m)
	case *ExternalizeMessage:
		cs.bState.Handle(sender, m)
	default:
		log.Printf("unrecognized message: %v", m)
	}

	cs.AssertValid()
}

