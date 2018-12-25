package main

import (
	//"bytes"
    //"math/rand"
    //"fmt"
	"os/user"
    "path"
    "time"
    //"ioutil"

    "testing"

	"github.com/plan-systems/go-plan/ski"
	"github.com/plan-systems/go-plan/plan"

    "github.com/plan-systems/go-plan/ski/Providers/fileRepo"

	"github.com/plan-systems/go-plan/ski/CryptoKits/nacl"

)





var gTesting *testing.T


func getTmpDir() string {

	usr, err := user.Current()
	if err != nil {
		gTesting.Fatal(err)
	}

	return path.Join(usr.HomeDir, "plan-testing")
}


func TestFileSysSKI(t *testing.T) {


    gTesting = t


   
    ski.RegisterProvider(fs.Provider)
    ski.RegisterCryptoKit(&nacl.CryptoKit)
    ski.RegisterCryptoKit(&nacl.CryptoKit)
    ski.RegisterCryptoKit(&nacl.CryptoKit)

    // Register providers to test 
    providersToTest := []string{}
    providersToTest = append(providersToTest, 
        fs.Provider.InvocationStr(),
    )

    for _, invocationStr := range providersToTest {

        invocation := plan.Block{
            Label: invocationStr,
        }

        A := newSession(invocation, "Alice")
        //B := newSession(invocation, "Bob")

        doCoreTests(A, A) //B)

        A.endSession("done A")
        //B.endSession("done B")
    }


}






type testSession struct {
    name        string
    session     ski.Session
    blocker     chan int   
    signingPubKey []byte
    encryptPubKey []byte
}




func (ts *testSession) doOp(inOpArgs ski.OpArgs) (*plan.Block, *plan.Perror) {

    var outErr *plan.Perror
    var outResults *plan.Block

    ts.session.DispatchOp(&inOpArgs, func(opResults *plan.Block, inErr *plan.Perror) {
        outErr = inErr
        outResults = opResults

        ts.blocker <- 1
    })

    <- ts.blocker

    return outResults, outErr
}





// test setup helper
func newSession(inInvocation plan.Block, inName string) *testSession {

    ts := &testSession{
        name:inName,
        blocker:make(chan int, 100),
    }


    userID := [6]byte{0, 1, 2, 3, 4, 5}
    commID := [6]byte{0, 0, 1, 1, 2, 2}

    var err *plan.Perror
    ts.session, err = ski.StartSession(ski.SessionParams{
        Invocation: inInvocation,
        UserID: userID[:],
        CommunityID: commID[:],
        BaseDir: getTmpDir(),
    })

    if err != nil {
        gTesting.Fatal(err)
    }

    ts.session.DispatchOp(&ski.OpArgs{
            OpName: ski.OpGenerateKeys,
            KeySpecs: ski.KeyBundle{
                CommunityId: commID[:],
                Keys: []*ski.KeyEntry{
                    &ski.KeyEntry{
                        KeyType: ski.KeyType_ASYMMETRIC_KEY,
                        KeyDomain: ski.KeyDomain_PERSONAL,
                    },
                    &ski.KeyEntry{
                        KeyType: ski.KeyType_SIGNING_KEY,
                        KeyDomain: ski.KeyDomain_PERSONAL,
                    },
                },
            },
        },
        func(inResults *plan.Block, inErr *plan.Perror) {
            if inErr == nil {
                bundleBuf := inResults.GetContentWithCodec(ski.KeyBundleProtobufCodec, 0)
                keyBundle := ski.KeyBundle{}
                err := keyBundle.Unmarshal(bundleBuf)
                if err != nil {
                    gTesting.Fatal(err)
                } 
                for i := 0; i < len(keyBundle.Keys); i++ {
                    switch keyBundle.Keys[i].KeyType {
                        case ski.KeyType_ASYMMETRIC_KEY:
                            ts.encryptPubKey = keyBundle.Keys[i].PubKey

                        case ski.KeyType_SIGNING_KEY:
                            ts.signingPubKey = keyBundle.Keys[i].PubKey
                    }
                }
            } else {
                gTesting.Fatal(inErr)
            }

            ts.blocker <- 1
        },
    )

    <- ts.blocker



    return ts
}



func (ts *testSession) endSession(inReason string) {

    ts.session.EndSession(inReason, func(inParam interface{}, inErr *plan.Perror) {
        if inErr != nil {
            gTesting.Fatal(inErr)
        }
        ts.blocker <- 1
    })

    <- ts.blocker

}




func doCoreTests(A, B *testSession) {

    time.Sleep(2000 * time.Millisecond)

}

/*

func doCoreTests(A, B *testSession) {

    // 1) make a new community key
	opResults, err := A.doOp(ski.OpArgs{
        OpName: ski.OpCreateSymmetricKey,
    })
	if err != nil {
		gTesting.Fatal(err)
    }
    communityKeyID := plan.GetKeyID(opResults.Content)

    fmt.Printf("%s's encryptPubKey %v\n", A.name, A.encryptPubKey)
    fmt.Printf("%s's encryptPubKey %v\n", B.name, B.encryptPubKey)

    // 2) generate a xfer community key msg from A
    opResults, err = A.doOp(ski.OpArgs{
        OpName: ski.OpSendKeys,
        OpKeyIDs: []plan.KeyID{communityKeyID},
        PeerPubKey: B.encryptPubKey,
        CryptoKeyID: A.encryptPubKeyID,
    })
    if err != nil {
        gTesting.Fatal(err)
    }
    
    // 3) insert the new community key into B
    opResults, err = B.doOp(ski.OpArgs{
        OpName: ski.OpAcceptKeys,
        Msg: opResults.Content,
        PeerPubKey: A.encryptPubKey,
        CryptoKeyID: B.encryptPubKeyID,
    })
    if err != nil {
        gTesting.Fatal(err)
    }

	clearMsg := []byte("hello, PLAN community!")

    // Encrypt a new community msg on A
	opResults, err = A.doOp(ski.OpArgs{
        OpName: ski.OpEncrypt,
        CryptoKeyID: communityKeyID,
        Msg: clearMsg,
    })
	if err != nil {
		gTesting.Fatal(err)
	}

    encryptedMsg := opResults.Content

    // Send the encrypted community message to B
	opResults, err = B.doOp(ski.OpArgs{
        OpName: ski.OpDecrypt,
        CryptoKeyID: communityKeyID,
        Msg: encryptedMsg,
    })
	if err != nil {
		gTesting.Fatal(err)
    }

	if ! bytes.Equal(clearMsg, opResults.Content) {
		gTesting.Fatalf("expected %v, got %v after decryption", clearMsg, opResults.Content)
    }
    

    badMsg := make([]byte, len(encryptedMsg))

    // Vary the data slightly to test 
    for i := 0; i < 1000; i++ {

        rndPos := rand.Int31n(int32(len(encryptedMsg)))
        rndAdj := 1 + byte(rand.Int31n(254))
        copy(badMsg, encryptedMsg)
        badMsg[rndPos] += rndAdj

        _, err = B.doOp(ski.OpArgs{
            OpName: ski.OpDecrypt,
            CryptoKeyID: communityKeyID,
            Msg: badMsg,
        })
        if err == nil {
            gTesting.Fatal("there should have been a decryption error!")
        }
    }
    
}













func (ts *testSession) doOp(inOpArgs ski.OpArgs) (*plan.Block, *plan.Perror) {

    var outErr *plan.Perror
    var outResults *plan.Block

    ts.session.DispatchOp(&inOpArgs, func(opResults *plan.Block, inErr *plan.Perror) {
        outErr = inErr
        outResults = opResults

        ts.blocker <- 1
    })

    <- ts.blocker

    return outResults, outErr
}




func (ts *testSession) endSession(inReason string) {

    ts.session.EndSession(inReason, func(inParam interface{}, inErr *plan.Perror) {
        if inErr != nil {
            gTesting.Fatal(inErr)
        }
        ts.blocker <- 1
    })

    <- ts.blocker

}



// test setup helper
func newSession(inInvocation plan.Block, inName string) *testSession {

    ts := &testSession{
        name:inName,
        blocker:make(chan int, 100),
    }

    var err *plan.Perror
    ts.session, err = ski.StartSession(
        inInvocation,
        ski.GatewayRWAccess,
        nil,
    )
    if err != nil {
        gTesting.Fatal(err)
    }

    identityResults, err := ts.doOp(
        ski.OpArgs{
            OpName: ski.OpNewIdentityRev,
        })
    if err != nil {
        gTesting.Fatal(err)
    }


    ts.signingKeyID = plan.GetKeyID( identityResults.GetContentWithLabel(ski.PubSigningKeyName) )

    ts.encryptPubKey = identityResults.GetContentWithLabel(ski.PubCryptoKeyName)
    ts.encryptPubKeyID = plan.GetKeyID( ts.encryptPubKey )

    return ts
}


*/