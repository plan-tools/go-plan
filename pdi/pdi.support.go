// See pdi.proto

package pdi

import (
	"github.com/plan-tools/go-plan/plan"

	"github.com/ethereum/go-ethereum/crypto/sha3"
)

// EntryAddr specifies the address of a PDI entry (an EntryCrypt) in a given StorageProvider.
// Since a StorageTxn can potentially hold more than one entry, EntryIndex specifies which one (using zero-based indexing).
type EntryAddr struct {
	TimeCommited int64
	TxnName      []byte
	EntryIndex   uint16
}

// EntryVersionMask is a bit mask on EntryCrypt.CryptInfo to extract pdi.EntryVersion
const EntryVersionMask = 0xFF

// GetEntryVersion returns the version of this entry (should match EntryVersion1)
func (entry *EntryCrypt) GetEntryVersion() EntryVersion {
	return EntryVersion(entry.CryptInfo & EntryVersionMask)
}

// ComputeHash hashes all fields of psi.EntryCrypt (except .EntrySig)
func (entry *EntryCrypt) ComputeHash() []byte {

	hw := sha3.NewKeccak256()

	var scrap [16]byte

	pos := 0
	pos = encodeVarintPdi(scrap[:], pos, entry.CryptInfo)

	hw.Write(scrap[:pos])
	hw.Write(entry.CommunityKeyId)
	hw.Write(entry.HeaderCrypt)
	hw.Write(entry.BodyCrypt)

	return hw.Sum(nil)

}

/*****************************************************
** Utils
**/

// MarshalForOptionalBody marshals txn so that it can be deserializaed via UnmarshalWithOptionalBody().
func (txn *StorageTxn) MarshalForOptionalBody(dAtA []byte) ([]byte, error) {

	// Store the body in a different segment so can load it optionally
	body := txn.Body
	txn.Body = nil

	// Make a scrap buffer big enough to hold StorageTxn (w/o a body) and the body  -- TODO: use a buffer pool
	headerSz := txn.Size()
	bodySz := body.Size()

	szNeeded := headerSz + bodySz + 32
	szAvail := cap(dAtA)
	if szAvail < szNeeded {
		dAtA = make([]byte, szNeeded+32000)
	} else {
		dAtA = dAtA[:szAvail]
	}

	var err error

	// Marshal the header, prepend the header byte length
	headerSz, err = txn.MarshalTo(dAtA[2:])
	dAtA[0] = byte((headerSz >> 1) & 0xFF)
	dAtA[1] = byte((headerSz) & 0xFF)
	if err == nil {
		bodySz, err = body.MarshalTo(dAtA[2+headerSz:])
		finalSz := 2 + headerSz + bodySz
		if finalSz < len(dAtA) {
			dAtA = dAtA[:finalSz]
		} else {
			err = plan.Error(err, plan.FailedToMarshal, "StorageTxn.MarshalWithOptionalBody() assert failed")
		}
	}

	return dAtA, err
}

// UnmarshalWithOptionalBody allows the caller to not unmarshal the body, saving on allocation and cycles
func (txn *StorageTxn) UnmarshalWithOptionalBody(dAtA []byte, inUnmarshalBody bool) error {
	dataLen := len(dAtA)
	if dataLen < 8 {
		return plan.Error(nil, plan.FailedToUnmarshal, "StorageTxn.UnmarshalWithOptionalBody() failed")
	}

	var headerSz uint
	headerSz = uint(dAtA[0]<<1) | uint(dAtA[1])
	err := txn.Unmarshal(dAtA[2 : 2+headerSz])
	if err != nil {
		return err
	}
	if inUnmarshalBody {
		if txn.Body == nil {
			txn.Body = &plan.Block{}
		}
		err = txn.Body.Unmarshal(dAtA[2+headerSz:])
	}

	return err

}

/*
// SegmentIntoTxnsForMaxSize is a utility that chops up a payload buffer into segments <= inMaxSegmentSize
func SegmentIntoTxnsForMaxSize(
	inData []byte,
	inDataDesc TxnDataDesc,
	inMaxSegmentSize int,
) ([]*StorageTxn, error) {

	bytesRemain := len(inData)
	pos := 0

	N := (len(inData) + inMaxSegmentSize - 1) / inMaxSegmentSize
	txns := make([]*StorageTxn, 0, N)

	for bytesRemain > 0 {

		segSz := bytesRemain
		if segSz < inMaxSegmentSize {
			segSz = inMaxSegmentSize
		}

		txns = append(txns, &StorageTxn{
			TxnStatus:   TxnStatus_AWAITING_COMMIT,
			DataDesc:    inDataDesc,
			SegmentData: inData[pos:segSz],
		})

		pos += segSz
	}

	for i, txn := range txns {
		txn.SegmentNum = uint32(i)
		txn.TotalSegments = uint32(len(txns))
	}

	return txns, nil

	//if bytesRemain != 0 {
	//    return plan.Error(nil, plan.AssertFailure, "assertion failed in SegmentPayloadForSegmentSize {N:%d, bytesRemain:%d}", N, bytesRemain)
	//}
}
*/