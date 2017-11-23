package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ethereum/go-ethereum/crypto"
	"fmt"
)

func Test_CreateNewIdentity(t *testing.T) {
	manager := NewIdentityManager("testdata")
	id, err := manager.CreateNewIdentity()
	assert.NoError(t, err)
	assert.Len(t, id, 42)
}

func Test_GetIdentities(t *testing.T) {
	manager := NewIdentityManager("testdata")
	ids := manager.GetIdentities()
	for _, id := range ids {
		fmt.Println(id)
	}
}

func Test_GetIdentity(t *testing.T) {
	manager := NewIdentityManager("testdata")
	ids := manager.GetIdentities()
	for _, id := range ids {
		identity := manager.GetIdentity(string(id))
		assert.NotNil(t, identity)
		assert.Equal(t, id, *identity)
	}

	identity := manager.GetIdentity("")
	assert.Nil(t, identity)
}

func Test_HasIdentity(t *testing.T) {
	manager := NewIdentityManager("testdata")
	ids := manager.GetIdentities()
	for _, id := range ids {
		assert.True(t, manager.HasIdentity(string(id)))
	}

	identity := manager.HasIdentity("")
	assert.False(t, identity)
}

func Test_SignMessage(t *testing.T) {
	manager := NewIdentityManager("testdata")
	ids := manager.GetIdentities()
	for _, id := range ids {
		signature, err := manager.SignMessage(id, "message to sign")
		assert.NoError(t, err)
		assert.Len(t, signature, 65)
	}
}
func Test_SignVerifyMessage(t *testing.T) {

	key, err := crypto.GenerateKey()
	assert.NoError(t, err)
	message := []byte("message to sign")

	signature, err := crypto.Sign(signHash(message), key)
	assert.NoError(t, err)

	rpk, err := crypto.Ecrecover(signHash(message), signature)
	assert.NoError(t, err)
	pubKey := crypto.ToECDSAPub(rpk)
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)

	assert.Equal(t, recoveredAddr, crypto.PubkeyToAddress(key.PublicKey))

}