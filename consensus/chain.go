package consensus

import (
	"log"

	"github.com/davecgh/go-spew/spew"

	"coinkit/util"
)

// Chain creates the blockchain, gaining consensus on one Block at a time.
// Chain is not threadsafe.
type Chain struct {
	// The block we are currently working on
	current *Block

	// Maps slot number to previous blocks
	// Every block in here should be externalized
	history map[int]*Block

	// The quorum logic we use for future blocks
	D QuorumSlice

	// Who we are
	publicKey string

	values ValueStore
}

func (c *Chain) Logf(format string, a ...interface{}) {
	util.Logf("CH", c.publicKey, format, a...)
}

// Handle handles an incoming message.
// It may return a message to be sent back to the original sender, or it may
// just return nil if it has no particular response.
func (c *Chain) Handle(sender string, message util.Message) util.Message {
	if sender == c.publicKey {
		// It's one of our own returning to us, we can ignore it
		return nil
	}

	slot := message.Slot()
	if slot == 0 {
		log.Fatalf("slot should not be zero in %s", message)
	}

	// Handle info messages
	if _, ok := message.(*util.InfoMessage); ok {
		block := c.history[slot]
		if block != nil {
			return block.external
		}
		return nil
	}

	if slot == c.current.slot {
		c.current.Handle(sender, message)
		if c.current.Done() && c.values.CanFinalize(c.current.external.X) {
			// This block is done, let's move on to the next one
			c.Logf("advancing to slot %d", slot+1)
			c.values.Finalize(c.current.external.X)
			c.history[slot] = c.current
			c.current = NewBlock(c.publicKey, c.D, slot+1, c.values)
		}
		return nil
	}

	// This message is for an old block
	if _, ok := message.(*ExternalizeMessage); ok {
		// The sender is done with this block and so are we
		return nil
	}

	// The sender is behind. Let's send them info for the old block
	oldBlock := c.history[slot]
	if oldBlock != nil {
		c.Logf("sending a catchup for slot %d", oldBlock.external.I)
		return oldBlock.external
	}

	// We can't help the sender catch up
	return nil
}

func (c *Chain) AssertValid() {
	c.current.AssertValid()
}

// Slot() returns the slot this chain is currently working on
func (c *Chain) Slot() int {
	return c.current.slot
}

func NewEmptyChain(publicKey string, qs QuorumSlice, vs ValueStore) *Chain {
	return &Chain{
		current:   NewBlock(publicKey, qs, 1, vs),
		history:   make(map[int]*Block),
		D:         qs,
		values:    vs,
		publicKey: publicKey,
	}
}

// ValueStoreUpdated should be called when the value store is updated
func (c *Chain) ValueStoreUpdated() {
	c.current.ValueStoreUpdated()
}

func (c *Chain) OutgoingMessages() []util.Message {
	answer := c.current.OutgoingMessages()

	prev := c.history[c.current.slot-1]
	if prev != nil {
		// We also send out the externalize data for the previous block
		answer = append(answer, prev.OutgoingMessages()...)
	}

	return answer
}

func (chain *Chain) Stats() {
	chain.Logf("%d blocks externalized", chain.Slot()-1)
}

func (chain *Chain) Log() {
	log.Printf("--------------------------------------------------------------------------")
	log.Printf("%s is working on slot %d", chain.publicKey, chain.current.slot)
	if chain.current.bState != nil {
		chain.current.bState.Show()
	} else {
		log.Printf("spew: %s", spew.Sdump(chain))
	}
}

func LogChains(chains []*Chain) {
	for _, chain := range chains {
		chain.Log()
	}
	log.Printf("**************************************************************************")
}
