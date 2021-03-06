package consensus

import (
	"strings"
	
	"coinkit/util"
)

// The nomination message format of the Stellar Consensus Protocol.
// Implements Message.
// See:
// https://www.stellar.org/papers/stellar-consensus-protocol.pdf
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
	return "N"
}

func (m *NominationMessage) Slot() int {
	return m.I
}

func (m *NominationMessage) String() string {
	shortNom := []string{}
	shortAcc := []string{}
	for _, nom := range m.Nom {
		shortNom = append(shortNom, util.Shorten(string(nom)))
	}
	for _, acc := range m.Acc {
		shortAcc = append(shortAcc, util.Shorten(string(acc)))
	}
	answer := "nominate []"
	if len(shortNom) > 0 {
		answer = "nominate " + strings.Join(shortNom, ",")
	}
	if len(shortAcc) > 0 {
		answer += " accept " + strings.Join(shortAcc, ",")
	}
	return answer
}

func init() {
	util.RegisterMessageType(&NominationMessage{})
}
