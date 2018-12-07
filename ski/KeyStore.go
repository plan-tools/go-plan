package ski

import (
    "sync"
	crypto_rand "crypto/rand"

	"github.com/plan-systems/go-plan/plan"

)



type keyring struct {
    KeysByID        map[plan.KeyID]*KeyEntry
}



type KeyringSet struct {
    sync.RWMutex

    ByKeyDomain     [NumKeyDomains]keyring
}



func (ks *KeyringSet) GenerateNewKeys(
    inKeyReqs []*KeyEntry,
) ([]*KeyEntry, *plan.Perror) {

    for {

        newKeys, err := GenerateNewKeys(crypto_rand.Reader, 32, inKeyReqs)
        if err != nil {
            return nil, err
        }

        // Let's all laugh and be merry at the possibility of a 1:2^256 collision!  I want a pony and air-jammer-road-rammer!
        keysNotMerged := ks.MergeKeys(newKeys)
        if len(keysNotMerged) == 0 {
            return newKeys, nil
        }
    }
   

}


// ExportNamedKeys returns a list of KeyEntries, where each element corresponds to the elements in inKeySpecs. 
// If inErrorOnKeyNotFound is set and a KeySpec isn't found, an error is returned.  Otherwise, nil is returned for that element.
func (ks *KeyringSet) ExportNamedKeys(
    inKeySpecs []*KeyEntry,
    inErrorOnKeyNotFound bool,
) ([]*KeyEntry, *plan.Perror) {

    outKeys := make([]*KeyEntry, len(inKeySpecs))

    ks.RLock()
    defer ks.RUnlock()

    for i, keySpec := range inKeySpecs {

        var err *plan.Perror
        var keyEntry *KeyEntry

        if keySpec.KeyDomain < 0 || keySpec.KeyDomain > NumKeyDomains {
            err = plan.Errorf(nil, plan.KeyDomainNotFound, "key domain not found {KeyDomain: %v}", keySpec.KeyDomain)
        }
        
        if err == nil {
            keyID := plan.GetKeyID(keySpec.PubKey)

            keyEntry = ks.ByKeyDomain[keySpec.KeyDomain].KeysByID[keyID]
            if inErrorOnKeyNotFound && keyEntry == nil {
                err = plan.Errorf(nil, plan.KeyIDNotFound, "one or more keys not found {PubKey:%v}", keySpec.PubKey)
            }

            outKeys[i] = keyEntry
        }

        // If we don't error out, nil is left in elements, allowing the calling to know what keys weren't found
        if err != nil && inErrorOnKeyNotFound {
            return nil, err
        }
   }

   return outKeys, nil
}

// ExportKeyring returns an entire KeyDomain
func (ks *KeyringSet) ExportKeyring(
    inKeyDomain KeyDomain,
) ([]*KeyEntry, *plan.Perror) {

    if inKeyDomain < 0 || inKeyDomain > NumKeyDomains {
        return nil, plan.Errorf(nil, plan.KeyDomainNotFound, "keyring not found {KeyDomain: %v}", inKeyDomain)
    }

    ks.RLock()
    
    keysByID := ks.ByKeyDomain[inKeyDomain].KeysByID
    outKeys := make([]*KeyEntry, len(keysByID), 0)
    for _, keyEntry := range keysByID {
        outKeys = append(outKeys, keyEntry)
    }

    ks.RUnlock()

    // TODO: keys sort by time

   return outKeys, nil
}

/*
func SerializeKeysAsBundle(
    inKeys []*KeyEntry,
) ([]byte, *plan.Perror) {

    bundle := KeyBundle{
        Keys: inKeys
    }
}


    keys, err := ks.ExportKeyring(inKeyDomain)
    if err != nil {
        return nil, err
    }
    if inKeyDomain < 0 || inKeyDomain > NumKeyDomains {
        return nil, plan.Errorf(nil, plan.KeyDomainNotFound, "keyring not found {KeyDomain: %v}", inKeyDomain)
    }

    ks.RLock()
    
    keysByID := ks.ByKeyDomain[inKeyDomain].KeysByID
    outKeys := make([]*KeyEntry, len(keysByID), 0)
    for _, keyEntry := range keysByID {
        outKeys = append(outKeys, keyEntry)
    }

    ks.RUnlock()

    // TODO: keys sort by time

   return outKeys, nil
}
*/



// MergeKeys adds the keys to this KeyringSet returning a list of the collisions not merged.
// If an incoming collides (and isn't exactly identical to the existing entry), 
//    then it is added to list of keys returned (i.e. this func merges the keys it can).
func (ks *KeyringSet) MergeKeys(
    inKeysToMerge []*KeyEntry,
) []*KeyEntry {

    var keysNotMerged []*KeyEntry
        
    ks.Lock()
    for _, keyEntry := range inKeysToMerge {

        if keyEntry != nil {
            merged := false
            if keyEntry.KeyDomain >= 0 && keyEntry.KeyDomain < NumKeyDomains {

                keyID := keyEntry.GetKeyID()
                keyring := &ks.ByKeyDomain[keyEntry.KeyDomain]
                existing := keyring.KeysByID[keyID]
                if existing == nil || existing.EqualTo(keyEntry) {
                    merged = true
                    if existing == nil {
                        keyring.KeysByID[keyID] = keyEntry
                    }
                }
            }
            
            if ! merged {
                keysNotMerged = append(keysNotMerged, keyEntry)
            }
        }
    }
    ks.Unlock()

/*
    if len(collisions) > 0 {
        err = plan.Errorf(nil, plan.KeyIDCollision, "key ID collision while adding keys {keyID:%v}", collisions)
    }*/

    return keysNotMerged

}

// KeyStore is a basic memory-resident data structure that stores KeyEntry, placed in a hierarchy of data structures, indexed by:
//    CommunityID => KeyDomain => KeyEntry.PubKey
type KeyStore struct {
    sync.RWMutex

    ByCommunity   map[plan.CommunityID]*KeyringSet
}

// FetchKeyrings returns the KeyringSet associated with the given community ID 
func (keyStore *KeyStore) FetchKeyrings(
    inCommunityID []byte,
) (*KeyringSet, *plan.Perror) {

    CID := plan.GetCommunityID(inCommunityID)
 
    keyStore.RLock()
    keyringSet, ok := keyStore.ByCommunity[CID]
    keyStore.RUnlock()

    if ! ok {
        return nil, plan.Errorf(nil, plan.KeyringNotFound, "no keyrings found for community ID %v", inCommunityID)
    }

    return keyringSet, nil
}




/*

// ExportKeys exports the given list of keys into a buffer t
func (kr *Keyring) ExportKeys(
    inKeyIDs []plan.KeyID,
    ioKeyList *KeyList,
    ) []plan.KeyID {

    var keysNotFound []plan.KeyID
    
    kr.RLock()
    for _, keyID := range inKeyIDs {
        existing := kr.keysByID[keyID]
        if existing != nil {
            ioKeyList.Keys = append(ioKeyList.Keys, existing)
        } else {
            keysNotFound = append(keysNotFound, keyID)
        }
    }
    kr.RUnlock()

    return keysNotFound

}


// MergeKeys adds a key to the keychain (ignoring collitions if the key entry is identical)
func (kr *Keyring) MergeKeys(
    inKeyList KeyList,
    ) *plan.Perror {

    var collisions []*KeyEntry
    var keyID plan.KeyID
    
    kr.Lock()
    for _, entry := range inKeyList.Keys {
        keyID = entry.GetKeyID()
        existing := kr.keysByID[keyID]
        if existing != nil && ! existing.EqualTo(entry) {
            collisions = append(collisions, entry)
        } else {
            kr.keysByID[keyID] = entry
        }
    }
    kr.Unlock()

    var err *plan.Perror

    if len(collisions) > 0 {
        err = plan.Errorf(nil, plan.KeyIDCollision, "key ID collision while adding keys {keyID:%v}", collisions)
    }
    return err

}
*/

/*
func (kr *Keyring) GetKeyEntry(
    inKeyID plan.KeyID,
    outKeyEntry *KeyEntry,
    ) *plan.Perror {

    var keyEntry KeyEntry

	kr.RLock()
    keyEntry, ok := kr.keysByID[inKeyID]
    kr.RUnlock()
    
	if !ok {
		return plan.Errorf(nil, plan.KeyIDNotFound, "key not found {keyID:%v}", inKeyID)
    }
    
    *outKeyEntry = keyEntry

	return nil
}
*/