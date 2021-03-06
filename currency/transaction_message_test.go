package currency

import (
	"testing"

	"coinkit/util"
)

func TestTransactionMessages(t *testing.T) {
	kp1 := util.NewKeyPairFromSecretPhrase("key pair 1")
	kp2 := util.NewKeyPairFromSecretPhrase("key pair 2")
	t1 := Transaction{
		Sequence: 1,
		Amount: 100,
		Fee: 2,
		From: kp1.PublicKey(),
		To: kp2.PublicKey(),
	}
	t2 := Transaction{
		Sequence: 1,
		Amount: 50,
		Fee: 2,
		From: kp2.PublicKey(),
		To: kp1.PublicKey(),
	}
	s1 := t1.SignWith(kp1)
	s2 := t2.SignWith(kp2)
	message := NewTransactionMessage(s1, s2)

	m := util.EncodeThenDecode(message).(*TransactionMessage)
	if len(m.Transactions) != 2 {
		t.Fatal("expected len m.Transactions to be 2")
	}
	if !m.Transactions[0].Verify() {
		t.Fatal("expected m.Transactions[0].Verify()")
	}
	if !m.Transactions[1].Verify() {
		t.Fatal("expected m.Transactions[1].Verify()")
	}

}
