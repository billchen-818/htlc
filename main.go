package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

const (
	AccountChainCodeName = "account"
	AccountChainCodeChannel = "mychannel"

	HTLCPrefix = "HTLC-%s"
)

type HTLCChaincode struct {}

func (h *HTLCChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (h *HTLCChaincode) Invoke(stub shim.ChaincodeStubInterface) (res pb.Response) {
	defer func() {
		if r := recover(); r != nil {
			res = shim.Error(fmt.Sprintf("%v", r))
		}
	}()

	fn, args := stub.GetFunctionAndParameters()
	switch fn {
	case "create":
		res = h.create(stub, args)
	case "create_hash":
		res = h.createHash(stub, args)
	case "receive":
		res = h.receive(stub, args)
	case "refund":
		res = h.refund(stub, args)
	case "query_htlc":
		res = h.query(stub, args)
	default:
		res = shim.Error("Invalid invoke function name")
	}

	return
}

func (h *HTLCChaincode) create(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 6 {
		return shim.Error("arguments is invalid")
	}

	sender := args[0]
	receive := args[1]
	amountStr := args[2]
	timeLockStr := args[3]
	preImage := args[4]
	passwd := args[5]

	// 1�B?�dsender??�A�}�P???�O�_��?
	trans := [][]byte{[]byte("query"), []byte(sender)}
	resPonse := stub.InvokeChaincode(AccountChainCodeName, trans, AccountChainCodeChannel)
	if resPonse.Status != shim.OK {
		return shim.Success([]byte(resPonse.Message))
	}

	senderAccount := &Account{}
	if err := json.Unmarshal(resPonse.Payload, senderAccount); err != nil {
		return shim.Error(err.Error())
	}

	amount, err := stringToUint64(amountStr)
	if err != nil {
		return shim.Error(err.Error())
	}

	if amount > senderAccount.Amount {
		return shim.Error("account assert is not enough")
	}

	// 2�B?��?�w??
	str := senderAccount.Address+uint64ToString(senderAccount.Sequence)
	addressByte := sha256.Sum256([]byte(str))
	address := hex.EncodeToString(addressByte[:])

	trans = [][]byte{[]byte("create"), []byte(address), []byte(preImage), []byte("")}
	resPonse = stub.InvokeChaincode(AccountChainCodeName, trans, AccountChainCodeChannel)
	if resPonse.Status != shim.OK {
		return shim.Success([]byte(resPonse.Message))
	}

	// 3�B?�e��??�V?�w???��??
	trans = [][]byte{[]byte("transfer"), []byte(sender), []byte(address), []byte(amountStr), []byte(passwd)}
	resPonse = stub.InvokeChaincode(AccountChainCodeName, trans, AccountChainCodeChannel)
	if resPonse.Status != shim.OK {
		return shim.Success([]byte(resPonse.Message))
	}

	// 4�B?��htlc�}��^hashvalue
	hashValueBytes := sha256.Sum256([]byte(preImage))
	hashValue := hex.EncodeToString(hashValueBytes[:])

	timeLock, err := strconv.ParseInt(timeLockStr, 10, 64)
	if err != nil {
		return shim.Error(err.Error())
	}
	timeLock = time.Now().Unix() + timeLock

	htlc := HTLC{
		Sender: sender,
		Receiver: receive,
		Amount: amount,
		HashValue: hashValue,
		TimeLock: timeLock,
		PreImage: "",
		LockAddress: address,
		State : HashLOCK,
	}

	htlcByte, err := json.Marshal(htlc)
	idByte := sha256.Sum256(htlcByte)
	id := hex.EncodeToString(idByte[:])
	key := fmt.Sprintf(HTLCPrefix, id)

	if err := stub.PutState(key, htlcByte); err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success([]byte(id))
}

func (h *HTLCChaincode) createHash(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 6 {
		return shim.Error("arguments is invalid")
	}

	sender := args[0]
	receive := args[1]
	amountStr := args[2]
	timeLockStr := args[3]
	hashValue := args[4]
	passwd := args[5]

	// 1�B?�dsender??�A�}�P???�O�_��?
	trans := [][]byte{[]byte("query"), []byte(sender)}
	resPonse := stub.InvokeChaincode(AccountChainCodeName, trans, AccountChainCodeChannel)
	if resPonse.Status != shim.OK {
		return shim.Success([]byte(resPonse.Message))
	}

	senderAccount := &Account{}
	if err := json.Unmarshal(resPonse.Payload, senderAccount); err != nil {
		return shim.Error(err.Error())
	}

	amount, err := stringToUint64(amountStr)
	if err != nil {
		return shim.Error(err.Error())
	}

	if amount > senderAccount.Amount {
		return shim.Error("account assert is not enough")
	}

	// 2�B?�e��???��?�w??
	str := senderAccount.Address+uint64ToString(senderAccount.Sequence)
	addressByte := sha256.Sum256([]byte(str))
	address := hex.EncodeToString(addressByte[:])

	trans = [][]byte{[]byte("create"), []byte(address), []byte(hashValue), []byte("hash")}
	resPonse = stub.InvokeChaincode(AccountChainCodeName, trans, AccountChainCodeChannel)
	if resPonse.Status != shim.OK {
		return shim.Success([]byte(resPonse.Message))
	}

	// 3�B?�e��??�V?�w???��??
	trans = [][]byte{[]byte("transfer"), []byte(sender), []byte(address), []byte(amountStr), []byte(passwd)}
	resPonse = stub.InvokeChaincode(AccountChainCodeName, trans, AccountChainCodeChannel)
	if resPonse.Status != shim.OK {
		return shim.Success([]byte(resPonse.Message))
	}

	// 4�B?��htlc
	timeLock, err := strconv.ParseInt(timeLockStr, 10, 64)
	if err != nil {
		return shim.Error(err.Error())
	}

	timeLock = time.Now().Unix() + timeLock

	htlc := HTLC{
		Sender: sender,
		Receiver: receive,
		Amount: amount,
		HashValue: hashValue,
		TimeLock: timeLock,
		PreImage: "",
		LockAddress: address,
		State : HashLOCK,
	}

	htlcByte, err := json.Marshal(htlc)
	idByte := sha256.Sum256(htlcByte)
	id := hex.EncodeToString(idByte[:])
	key := fmt.Sprintf(HTLCPrefix, id)

	if err := stub.PutState(key, htlcByte); err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success([]byte(id))
}

func (h *HTLCChaincode) receive(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 2 {
		return shim.Error("argument is invalid")
	}

	id := args[0]
	preImage := args[1]

	// 1�B?�dhtlc
	key := fmt.Sprintf(HTLCPrefix, id)
	htlcByte, err := stub.GetState(key)
	if err != nil {
		return shim.Error(err.Error())
	}

	if htlcByte == nil {
		return shim.Error("not have this htlc transaction")
	}

	htlc := &HTLC{}
	if err = json.Unmarshal(htlcByte, htlc); err != nil {
		return shim.Error(err.Error())
	}

	if htlc.State != HashLOCK {
		return shim.Error("this htlc transaction state is error")
	}

	if htlc.TimeLock < time.Now().Unix() {
		return shim.Error("time is expirate")
	}

	// 2�B??�w????�챵����??
	trans := [][]byte{[]byte("transfer"), []byte(htlc.LockAddress), []byte(htlc.Receiver), []byte(uint64ToString(htlc.Amount)), []byte(preImage)}
	resPonse := stub.InvokeChaincode(AccountChainCodeName, trans, AccountChainCodeChannel)
	if resPonse.Status != shim.OK {
		return shim.Success([]byte(resPonse.Message))
	}

	// 3�B��shtlc,preImage�K�[�W
	htlc.PreImage = preImage
	htlc.State = Received
	htlcByte, err = json.Marshal(htlc)
	if err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.PutState(key, htlcByte); err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(nil)
}

func (h *HTLCChaincode) refund(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 2 {
		return shim.Error("argument is invalid")
	}

	id := args[0]
	preImage := args[1]

	// 1�B?�dhtlc
	key := fmt.Sprintf(HTLCPrefix, id)
	htlcByte, err := stub.GetState(key)
	if err != nil {
		return shim.Error(err.Error())
	}

	if htlcByte == nil {
		return shim.Error("not have this htlc transaction")
	}

	htlc := &HTLC{}
	if err = json.Unmarshal(htlcByte, htlc); err != nil {
		return shim.Error(err.Error())
	}

	if htlc.TimeLock > time.Now().Unix() {
		return shim.Error("time is not expirate")
	}

	// 2�B??�w????��?�e��??
	trans := [][]byte{[]byte("transfer"), []byte(htlc.LockAddress), []byte(htlc.Sender), []byte(uint64ToString(htlc.Amount)), []byte(preImage)}
	resPonse := stub.InvokeChaincode(AccountChainCodeName, trans, AccountChainCodeChannel)
	if resPonse.Status != shim.OK {
		return shim.Success([]byte(resPonse.Message))
	}

	// 3�B��shtlc
	htlc.State = Refund
	if htlcByte, err = json.Marshal(htlc); err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.PutState(key, htlcByte); err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(nil)
}

func (h *HTLCChaincode) query(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 1 {
		return shim.Error("argument is invalid")
	}

	key := fmt.Sprintf(HTLCPrefix, args[0])
	htlcByte, err := stub.GetState(key)
	if err != nil {
		return shim.Error(err.Error())
	}

	if htlcByte == nil {
		return shim.Error("not have this htlc transaction")
	}

	return shim.Success(htlcByte)
}

func stringToUint64(s string) (u uint64, err error) {
	u, err = strconv.ParseUint(s, 10, 64)
	return
}

func uint64ToString(u uint64) string {
	return strconv.FormatUint(u, 10)
}

func main() {
	err := shim.Start(new(HTLCChaincode))
	if err != nil {
		fmt.Printf("Error starting HTLC chaincode: %s", err)
	}
}