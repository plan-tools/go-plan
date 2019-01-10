// See pdi.proto

package pdi

import (

    "io"

	"github.com/plan-systems/go-plan/plan"
	//"github.com/plan-systems/go-plan/pdi"

    "golang.org/x/crypto/sha3"

)

/*
// EntryAddr specifies the address of a PDI entry (an EntryCrypt) in a given StorageProvider.
// Since a StorageTxn can potentially hold more than one entry, EntryIndex specifies which one (using zero-based indexing).
type EntryAddr struct {
	TimeCommited int64
	TxnName      []byte
	EntryIndex   uint16
}
*/

// EntryVersionMask is a bit mask on EntryCrypt.CryptInfo to extract pdi.EntryVersion
const EntryVersionMask = 0xFF

// GetEntryVersion returns the version of this entry (should match EntryVersion1)
func (entry *EntryCrypt) GetEntryVersion() EntryVersion {
	return EntryVersion(entry.CryptInfo & EntryVersionMask)
}

// ComputeHash hashes all fields of psi.EntryCrypt (except .EntrySig)
func (entry *EntryCrypt) ComputeHash() []byte {

	hw := sha3.NewLegacyKeccak256()

	var scrap [16]byte

	pos := 0
	pos = encodeVarintPdi(scrap[:], pos, entry.CryptInfo)

	hw.Write(scrap[:pos])
	hw.Write(entry.CommunityKeyId)
	hw.Write(entry.HeaderCrypt)
	hw.Write(entry.BodyCrypt)

	return hw.Sum(nil)

}

// MarshalToBlock marshals this EntryCrypt into a generic plan.Block
func (entry *EntryCrypt) MarshalToBlock() *plan.Block {

    block := &plan.Block{
        CodecCode: plan.CodecCodeForEntryCrypt,
    }

    var err error
    block.Content, err = entry.Marshal()
    if err != nil {
        panic(err)
    }

    return block
}

/*****************************************************
** Utils
**/

/*
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



// UnmarshalEntries unmarshals txn.Body (created via MarshalEntries) into the EntryCrypts contained within it 
func (txn *StorageTxn) UnmarshalEntries(ioBatch []*EntryCrypt) ([]*EntryCrypt, error) {

    var err error

    N := len(txn.Body.Subs)

    for i := -1; i < N && err != nil ; i++ {

        var block *plan.Block
        if i == -1 {
            block = txn.Body
        } else {
            block = txn.Body.Subs[i]
        }
        if block.CodecCode == plan.CodecCodeForEntryCrypt {
            entry := &EntryCrypt{}
            err = entry.Unmarshal(block.Content)
            if err != nil {
                break
            }
            ioBatch = append(ioBatch, entry)
        }
    }

    return ioBatch, err
}
*/

// MarshalEntries marshals the given batch of entries into a single plan.Block
func MarshalEntries(inBatch []*EntryCrypt) *plan.Block {
    N := len(inBatch)

    var head *plan.Block

    if N == 1 {
        head = inBatch[0].MarshalToBlock()
    } else if N > 1 {
        
        head := &plan.Block{
            Subs: make([]*plan.Block, N),
        }
        for i := range inBatch {
            head.Subs[i] = inBatch[i].MarshalToBlock()
        }
    }

    return head
}


// WriteVarInt appends the given integer in variable length format
func WriteVarInt(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}

// AppendVarBuf appends the given buffer's length in bytes and the buffer
func AppendVarBuf(dAtA []byte, offset int, inBuf []byte) (int, error) {
    bufLen := len(inBuf)
    origOffset := offset
    offset = WriteVarInt(dAtA, offset, uint64(bufLen))

    remain := len(dAtA) - offset
    if remain < bufLen {
        return origOffset, io.ErrUnexpectedEOF
    }
    copy(dAtA[offset:], inBuf)
    return offset + bufLen, nil
}




// ReadVarBuf reads a buffer written by AppendVarBuf() and returns the offset
func ReadVarBuf(dAtA []byte, offset int) (int, []byte, error) {
	l := len(dAtA)
    
    var bufLen uint64
    for shift := uint(0); ; shift += 7 {
        if shift >= 31 {
            return offset, nil, ErrIntOverflowPdi
        }
        if offset >= l {
            return offset, nil, io.ErrUnexpectedEOF
        }
        b := dAtA[offset]
        offset++
        bufLen |= (uint64(b) & 0x7F) << shift
        if b < 0x80 {
            break
        }
    }

    start := offset
    offset += int(bufLen)

   if bufLen < 0 {
        return offset, nil, ErrInvalidLengthPdi
    }

    if offset > l {
        return  offset, nil, io.ErrUnexpectedEOF
    }

    return offset, dAtA[start:offset], nil
}




/*****************************************************
** Support
**/

/*
var storageMsgPool = sync.Pool{
    New: func() interface{} {
        return new(StorageMsg)
    },
}

// RecycleStorageMsg effectively deallocates the item and makes it available for reuse
func RecycleStorageMsg(inMsg *StorageMsg) {
    for _, txn := range inMsg.Txns {
        txn.Body = nil  // TODO: recycle plan.Blocks too
    }
    storageMsgPool.Put(inMsg)
}

// NewStorageMsg allocates a new StorageMsg
func NewStorageMsg() *StorageMsg {

    msg := storageMsgPool.Get().(*StorageMsg)
    if msg == nil {
        msg = &StorageMsg{}
    } else {
        msg.Txns = msg.Txns[:0]
        msg.AlertCode = 0
        msg.AlertMsg = ""
    }

    return msg
}

// NewStorageAlert creates a new storage msg with the given alert params
func NewStorageAlert(
    inAlertCode AlertCode, 
    inAlertMsg string,
    ) *StorageMsg {

    msg := NewStorageMsg()
    msg.AlertCode = inAlertCode
    msg.AlertMsg = inAlertMsg

    return msg 

}


*/




// SegmentIntoTxns is a utility that chops up a payload buffer into segments <= inMaxSegmentSize
func SegmentIntoTxns(
	inData           []byte,
    inPayloadName    []byte,
    inPayloadCodec   PayloadCodec, 
	inMaxSegmentSize int,
) ([]*TxnSegment, *plan.Perror) {

	bytesRemain := len(inData)
	pos := 0

	N := (len(inData) + inMaxSegmentSize - 1) / inMaxSegmentSize
	txns := make([]*TxnSegment, 0, N)

	for bytesRemain > 0 {

		segSz := bytesRemain
		if segSz > inMaxSegmentSize {
			segSz = inMaxSegmentSize
		}

		txns = append(txns, &TxnSegment{
            SegInfo: &TxnSegInfo{
                PayloadCodec: inPayloadCodec,
                PayloadName: inPayloadName,
                PayloadSize: int32(segSz),
            },
			SegData: inData[pos:pos+segSz],
		})

		pos += segSz
        bytesRemain -= segSz
	}

	for i, txn := range txns {
		txn.SegInfo.SegmentNum = uint32(i)
		txn.SegInfo.TotalSegments = uint32(len(txns))
	}

    plan.Assert(bytesRemain == 0, "assertion failed in SegmentIntoTxns {N:%d, bytesRemain:%d}", N, bytesRemain)

	return txns, nil

}



//func AssembleSegments(inSegs []*TxnSegment) ([]*TxnSegment, *plan.Perror)