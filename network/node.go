package network

import (
	"log"

	"coinkit/consensus"
	"coinkit/currency"
	"coinkit/data"
	"coinkit/util"
)

// Node is the logical container for everything one node in the network handles.
// Node is not threadsafe.
type Node struct {
	publicKey util.PublicKey
	chain     *consensus.Chain
	queue     *currency.TransactionQueue
	store     *data.DataStore
}

func NewNode(publicKey util.PublicKey, qs consensus.QuorumSlice) *Node {
	queue := currency.NewTransactionQueue(publicKey)

	return &Node{
		publicKey: publicKey,
		chain:     consensus.NewEmptyChain(publicKey, qs, queue),
		queue:     queue,
		store:     data.NewDataStore(),
	}
}

// Slot() returns the slot this node is currently working on
func (node *Node) Slot() int {
	return node.chain.Slot()
}

func (node *Node) DataStore() *data.DataStore {
	return node.store
}

// Handle handles an incoming message.
// It may return a message to be sent back to the original sender
// The bool flag tells whether it has a response or not.
func (node *Node) Handle(sender string, message util.Message) (util.Message, bool) {
	if sender == node.publicKey.String() {
		return nil, false
	}
	switch m := message.(type) {

	case *data.DataMessage:
		response := node.store.Handle(m)
		if response == nil {
			return nil, false
		}
		return response, true

	case *HistoryMessage:
		node.Handle(sender, m.T)
		node.Handle(sender, m.E)
		return nil, false

	case *currency.AccountMessage:
		return nil, false

	case *util.InfoMessage:
		if m.Account != "" {
			answer := node.queue.HandleInfoMessage(m)
			return answer, answer != nil
		}
		if m.I != 0 {
			answer, ok := node.chain.Handle(sender, m)
			return answer, ok
		}
		return nil, false

	case *currency.TransactionMessage:
		if node.queue.HandleTransactionMessage(m) {
			node.chain.ValueStoreUpdated()
		}
		return nil, false

	case *consensus.NominationMessage:
		answer, ok := node.handleChainMessage(sender, m)
		return answer, ok
	case *consensus.PrepareMessage:
		answer, ok := node.handleChainMessage(sender, m)
		return answer, ok
	case *consensus.ConfirmMessage:
		answer, ok := node.handleChainMessage(sender, m)
		return answer, ok
	case *consensus.ExternalizeMessage:
		answer, ok := node.handleChainMessage(sender, m)
		return answer, ok

	default:
		log.Printf("unrecognized message: %+v", m)
		return nil, false
	}
}

// A helper to handle the messages
func (node *Node) handleChainMessage(sender string, message util.Message) (util.Message, bool) {
	response, ok := node.chain.Handle(sender, message)
	if !ok {
		return nil, false
	}

	externalize, ok := response.(*consensus.ExternalizeMessage)
	if !ok {
		return response, true
	}

	// Augment externalize messages into history messages
	t := node.queue.OldChunkMessage(externalize.I)
	return &HistoryMessage{
		T: t,
		E: externalize,
		I: externalize.I,
	}, true
}

func (node *Node) OutgoingMessages() []util.Message {
	answer := []util.Message{}
	sharing := node.queue.TransactionMessage()
	if sharing != nil {
		answer = append(answer, sharing)
	}
	d := node.store.DataMessage()
	if d != nil {
		answer = append(answer, d)
	}
	for _, m := range node.chain.OutgoingMessages() {
		answer = append(answer, m)
	}
	return answer
}

func (node *Node) Stats() {
	node.chain.Stats()
	node.queue.Stats()
}

func (node *Node) Log() {
	node.chain.Log()
	node.queue.Log()
}
