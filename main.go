package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

const (
	HTLC_CREATE  HTLCState = 1
	HTLC_RECEIVE HTLCState = 2
	HTLC_REFUND  HTLCState = 3

	KEY_HTLCS = "key_htlcs"
)

type HTLCState int

type HTLC struct {
	ID             string    `json:"id"`
	Sender         string    `json:"sender"`
	ToAddr         string    `json:"to_addr"`
	Token          int       `json:"token"`
	Secret         []byte    `json:"secret"`
	CreateTime     int64     `json:"create_time"`
	ExpirationTime int64     `json:"expiration_time"`
	SouceCode      string    `json:"souce_code"`
	State          HTLCState `json:"state"`
}

type HTLCChaincode struct {
	HTLCS map[string]HTLC `json:"htlcs"`
}

type Account struct {
	Address string `json:"address"`
	Token   int    `json:"token"`
	LockToken int `json:"lock_token"`
}

func getRandomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

func (h *HTLCChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (h *HTLCChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	switch function {
	case "createaccount":
		return h.createAccount(stub, args)
	case "queryaccount":
		return h.queryAccount(stub, args)
	case "createhtlcbycode":
		return h.createHTLCByCode(stub, args)
	case "createhtlcbyhash":
		return h.createHTLCByHash(stub, args)
	case "invokeHTLC":
		return h.invokeHTLC(stub, args)
	case "refundhtlc":
		return h.refundHTLC(stub, args)
	case "queryhtlc":
		return h.queryHTLC(stub, args)
	default:
		return shim.Error("Invalid invoke function name")
	}
}

func (h *HTLCChaincode) createAccount(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	address := args[0]
	token, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Printf("create account string to int error: %v\n", err)
		shim.Error(err.Error())
	}
	acc := Account{
		Address: address,
		Token: token,
		LockToken: 0,
	}
	accByte, err := json.Marshal(acc)
	if err != nil {
		fmt.Printf("create account json marshal error: %v\n", err)
		return shim.Error(err.Error())
	}
	fmt.Println("address: ", address)
	fmt.Println("account: ", string(accByte))

	if err = stub.PutState(address, accByte);err != nil {
		fmt.Printf("create account putstate error: %v\n", err)
		return shim.Error(err.Error())
	}
	return shim.Success(nil)
}

func (h *HTLCChaincode) queryAccount(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	fmt.Println("address: ", args[0])
	accByte, err := stub.GetState(args[0])
	if err != nil {
		fmt.Printf("query account getstate error: %v\n", err)
		return shim.Error(err.Error())
	}

	fmt.Println("account: ", string(accByte))

	var acc Account
	if err = json.Unmarshal(accByte, &acc); err != nil {
		fmt.Printf("query account json unmarshal error:%v\n", err)
		return shim.Error(err.Error())
	}
	fmt.Printf("Query Account:\nAddress:%v\nToken:%v\nLockToken:%v\n",acc.Address, acc.Token, acc.LockToken)

	return shim.Success(accByte)
}

type ResponseCreateHTLC struct {
	ID string `json:"id"`
	Hash string `json:"hash"`
}

func (h *HTLCChaincode) createHTLCByCode(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	sender := args[0]
	to := args[1]
	tokenStr := args[2]
	souce := args[3]
	ttlStr := args[4]

	token, err := strconv.Atoi(tokenStr)
	if err != nil {
		fmt.Printf("create htlc tx token string to int error: %v\n", err)
		return shim.Error(err.Error())
	}

	hashValue := sha256.New().Sum([]byte(souce))

	creatime := time.Now()
	ttl, err := strconv.Atoi(ttlStr)
	if err != nil {
		fmt.Printf("create htlc tx ttl string to int error: %v\n", err)
		return shim.Error(err.Error())
	}

	expiration := creatime.Add(time.Duration(ttl)).Unix()

	htlc := HTLC{
		Sender: sender,
		ToAddr: to,
		Token: token,
		Secret: hashValue,
		CreateTime: creatime.Unix(),
		ExpirationTime: expiration,
		SouceCode: "",
		State: HTLC_CREATE,
	}

	htlcsByte, err := stub.GetState(KEY_HTLCS)
	if err != nil {
		fmt.Printf("create htlc tx get state 1 error: %v\n", err)
		return shim.Error(err.Error())
	}

	var htlcs HTLCChaincode

	htlcs.HTLCS = make(map[string]HTLC)

	fmt.Println(htlcsByte==nil)

	if htlcsByte != nil {
		err = json.Unmarshal(htlcsByte, &htlcs)
		fmt.Println("h: ", string(htlcsByte))
		if err != nil {
			fmt.Printf("create htlc tx json error: %v\n", err)
			return shim.Error(err.Error())
		}
	}

	id := getRandomString(20)
	if _, ok := htlcs.HTLCS[id]; ok {
		id = getRandomString(20)
	}
	htlc.ID = id
	htlcs.HTLCS[id] = htlc

	newHTLCsByte, err := json.Marshal(htlcs)
	if err != nil {
		return shim.Error(err.Error())
	}

	if err = stub.PutState(KEY_HTLCS, newHTLCsByte); err != nil {
		return shim.Error(err.Error())
	}

	accByte, err := stub.GetState(sender)
	if err != nil {
		return shim.Error(err.Error())
	}

	var sendAccount Account
	if err = json.Unmarshal(accByte, &sendAccount); err != nil {
		return shim.Error(err.Error())
	}

	sendAccount.LockToken = token

	newSendAccountByte, err := json.Marshal(sendAccount)
	if err != nil {
		return shim.Error(err.Error())
	}

	err = stub.PutState(sender, newSendAccountByte)
	if err != nil {
		return shim.Error(err.Error())
	}

	ret := ResponseCreateHTLC{
		ID: id,
		Hash: hex.EncodeToString(hashValue),
	}

	retByte, err := json.Marshal(ret)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(retByte)
}

type ResponseCreateHTLCByHash struct {
	ID string `json:"id"`
}

func (h *HTLCChaincode) createHTLCByHash(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	sender := args[0]
	to := args[1]
	tokenStr := args[2]
	haseValue := args[3]
	ttlStr := args[4]

	token, err := strconv.Atoi(tokenStr)
	if err != nil {
		return shim.Error(err.Error())
	}

	hash, err := hex.DecodeString(haseValue)
	if err != nil {
		return shim.Error(err.Error())
	}

	ttl, err := strconv.Atoi(ttlStr)
	if err != nil {
		return shim.Error(err.Error())
	}

	htlc := HTLC{
		Sender: sender,
		ToAddr: to,
		Token: token,
		Secret: hash,
		SouceCode: "",
	}

	creatime := time.Now()
	expiration := creatime.Add(time.Duration(ttl)).Unix()
	htlc.CreateTime = creatime.Unix()
	htlc.ExpirationTime = expiration
	htlc.State = HTLC_CREATE

	htlcsByte, err := stub.GetState(KEY_HTLCS)
	if err != nil {
		return shim.Error(err.Error())
	}

	var htlcs HTLCChaincode
	err = json.Unmarshal(htlcsByte, &htlcs)
	if err != nil {
		return shim.Error(err.Error())
	}

	id := getRandomString(20)
	if _, ok := htlcs.HTLCS[id]; ok {
		id = getRandomString(20)
	}
	htlc.ID = id
	htlcs.HTLCS[id] = htlc

	newHTLCsByte, err := json.Marshal(htlcs)
	if err != nil {
		return shim.Error(err.Error())
	}

	if err = stub.PutState(KEY_HTLCS, newHTLCsByte); err != nil {
		return shim.Error(err.Error())
	}

	accByte, err := stub.GetState(sender)
	if err != nil {
		return shim.Error(err.Error())
	}

	var sendAccount Account
	if err = json.Unmarshal(accByte, &sendAccount); err != nil {
		return shim.Error(err.Error())
	}

	sendAccount.LockToken = token

	newSendAccountByte, err := json.Marshal(sendAccount)
	if err != nil {
		return shim.Error(err.Error())
	}

	err = stub.PutState(sender, newSendAccountByte)
	if err != nil {
		return shim.Error(err.Error())
	}

	var ret ResponseCreateHTLCByHash
	ret.ID = id
	retByte, err := json.Marshal(ret)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(retByte)
}

func (h *HTLCChaincode) invokeHTLC(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	id := args[0]
	souce := args[1]

	htlcsByte, err := stub.GetState(KEY_HTLCS)
	if err != nil {
		return shim.Error(err.Error())
	}

	var htlcs HTLCChaincode
	err = json.Unmarshal(htlcsByte, &htlcs)
	if err != nil {
		return shim.Error(err.Error())
	}

	htlc, ok := htlcs.HTLCS[id]
	if !ok {
		return shim.Error("This transaction isnot exist!")
	}

	if !bytes.Equal(htlc.Secret, sha256.New().Sum([]byte(souce))) {
		return shim.Error("The souce is error")
	}

	if !time.Now().Before(time.Unix(htlc.ExpirationTime, 0)) {
		return shim.Error("This Transaction is expirated")
	}

	if htlc.State != HTLC_CREATE {
		return shim.Error("This Transaction State is error")
	}

	var toAccount Account
	toAccByte, err := stub.GetState(htlc.ToAddr)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(toAccByte, &toAccount)
	if err != nil {
		return shim.Error(err.Error())
	}
	toAccount.Token += htlc.Token
	newToAccountByte, err := json.Marshal(toAccount)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.PutState(htlc.ToAddr, newToAccountByte)
	if err != nil {
		return shim.Error(err.Error())
	}

	var sendAccount Account
	sendAccountByte, err := stub.GetState(htlc.Sender)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(sendAccountByte, &sendAccount)
	if err != nil {
		return shim.Error(err.Error())
	}
	sendAccount.Token -= htlc.Token
	sendAccount.LockToken -= htlc.Token
	newSendAccountByte, err := json.Marshal(sendAccount)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.PutState(htlc.Sender, newSendAccountByte)
	if err != nil {
		return shim.Error(err.Error())
	}

	htlc.State = HTLC_RECEIVE
	htlc.SouceCode = souce
	htlcs.HTLCS[id] = htlc
	newHTLCSByte, err := json.Marshal(htlcs)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.PutState(KEY_HTLCS, newHTLCSByte)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

func (h *HTLCChaincode) refundHTLC(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	id := args[0]

	var htlcs HTLCChaincode
	htlcsByte, err := stub.GetState(KEY_HTLCS)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(htlcsByte, &htlcs)
	if err != nil {
		return shim.Error(err.Error())
	}

	htlc, ok := htlcs.HTLCS[id]
	if !ok {
		return shim.Error("This transaction isnot exist!")
	}

	if !time.Now().After(time.Unix(htlc.ExpirationTime, 0)) {
		return shim.Error("This transaction is not expireted")
	}

	if htlc.State != HTLC_CREATE {
		return shim.Error("This Transaction state is not error")
	}

	var sendAccount Account
	accByte, err := stub.GetState(htlc.Sender)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(accByte, &sendAccount)
	if err != nil {
		return shim.Error(err.Error())
	}
	sendAccount.LockToken -= htlc.Token
	newAccByte, err := json.Marshal(sendAccount)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.PutState(htlc.Sender, newAccByte)
	if err != nil {
		return shim.Error(err.Error())
	}

	htlc.State = HTLC_REFUND
	htlcs.HTLCS[id] = htlc
	newHTLCByte, err := json.Marshal(htlcs)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = stub.PutState(KEY_HTLCS, newHTLCByte)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

func (h *HTLCChaincode) queryHTLC(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	id := args[0]

	var htlcs HTLCChaincode
	htlcsByte, err := stub.GetState(KEY_HTLCS)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(htlcsByte, &htlcs)
	if err != nil {
		return shim.Error(err.Error())
	}

	htlc, ok := htlcs.HTLCS[id]
	if !ok {
		return shim.Error("This transaction isnot exist!")
	}

	htlcByte, err := json.Marshal(htlc)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(htlcByte)
}

func main() {
	err := shim.Start(new(HTLCChaincode))
	if err != nil {
		fmt.Printf("Error starting HTLC chaincode: %s", err)
	}
}