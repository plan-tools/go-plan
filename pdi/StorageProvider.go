package pdi

import (
	//"fmt"
	//"sync"

	//"google.golang.org/grpc/encoding"

	"github.com/plan-systems/go-plan/ski"
)

/**********************************************************************************************************************
  StorageProvider wraps a persistent storage service provider.

  See the service defintion for StorageProvider, located in go-plan/pdi/pdi.proto.

  The TxnEncoder/TxnDecoder model is designed to wrap any kind of append-only database,
      particularly a distributed ledger.

*/

// TxnSegmentMaxSz allows malicious-sized txns to be detected
const TxnSegmentMaxSz = 10 * 1024 * 1024

// TxnEncoder encodes arbitrary data payloads into storage txns native to a specific StorageProvider.
// For each StorageProvider implementation, there is a corresponding TxnEncoder that allows
//     native txns to be generated locally for submission to the remote StorageProvider.
// The StorageProvider + txn Encoder/Decoder system preserves the property that a StorageProvider must operate deterministically,
//     and can only validate txns and maintain a ledger of which public keys can post (and how much).
// TxnEncoder is NOT assumed to be threadsafe unless specified otherswise
type TxnEncoder interface {

	// GenerateNewAccount gens the necessary key(s) in the given SKI session, in the keyring named ioKeyRef.KeyringName, 
    //    returning the newly generated pub key (used as an address) in ioKeyRef.PubKey.
	GenerateNewAccount(
        inSession ski.Session,
        ioKeyRef *ski.KeyRef,
    ) error

	// ResetSigner -- resets how this TxnEncoder signs newly encoded txns in EncodeToTxns().
	ResetSigner(
		inSession ski.Session,
		inFrom    ski.KeyRef,
	) error

	// EncodeToTxns encodes the payload and payload codec into one or more native and signed StorageProvider txns.
	// Pre: ResetSigner() must be successfully called.
	EncodeToTxns(
		inPayload      []byte,
		inPayloadCodec PayloadCodec,
		inTransfers    []*Transfer,
		inTimeSealed   int64, // If non-zero, this is used in place of the current time
	) ([]Txn, error)

	// Generates a txn that destroys the given address from committing any further txns.
	//EncodeDestruct(from ski.PubKey) (*Txn, error)
}

// TxnDecoder decodes storage txns native to a specific remote StorageProvider into the original payloads.
// TxnDecoder is NOT assumed to be threadsafes unless specified otherswise
type TxnDecoder interface {

	// EncodingDesc returns a string for use about this decoder
	EncodingDesc() string

	// Decodes a raw txn from a StorageProvider (from a corresponding TxnEncoder)
	// Also performs signature validation on the given txn, meaning that if no err is returned,
	//    then the txn was indeed signed by outInfo.From.
	// Returns the payload buffer segment buf.
	DecodeRawTxn(
		inRawTxn []byte,   // Raw txn to be decoded
		outInfo  *TxnInfo, // If non-nil, populated w/ info extracted from inTxn
	) ([]byte, error)
}
