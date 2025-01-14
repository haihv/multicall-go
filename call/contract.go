package call

import (
	"encoding/json"
	"github.com/depocket/multicall-go/core"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strings"
)

type Argument struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	InternalType string `json:"internalType"`
}

type Method struct {
	Name            string     `json:"name"`
	Inputs          []Argument `json:"inputs"`
	Outputs         []Argument `json:"outputs"`
	Type            string     `json:"type"`
	StateMutability string     `json:"stateMutability"`
}

type ContractBuilder interface {
	WithClient(ethClient *ethclient.Client) ContractBuilder
	AtAddress(contractAddress string) ContractBuilder
	AddMethod(signature string) ContractBuilder
	Abi() abi.ABI
	Build() *contract
}

type contract struct {
	ethClient   *ethclient.Client
	contractAbi abi.ABI
	rawMethods  map[string]string
	methods     []Method
	calls       []core.Call
	multiCaller *core.MultiCaller
}

func NewContractBuilder() ContractBuilder {
	return &contract{
		calls:      make([]core.Call, 0),
		methods:    make([]Method, 0),
		rawMethods: make(map[string]string, 0),
	}
}

func (a *contract) WithClient(ethClient *ethclient.Client) ContractBuilder {
	a.ethClient = ethClient
	return a
}

func (a *contract) Build() *contract {
	return a
}

func (a *contract) AtAddress(address string) ContractBuilder {
	caller, err := core.NewMultiCaller(a.ethClient, common.HexToAddress(address))
	if err != nil {
		panic(err)
	}
	a.multiCaller = caller
	return a
}

func (a *contract) AddCall(callName string, contractAddress string, method string, args ...interface{}) *contract {
	callData, err := a.contractAbi.Pack(method, args...)
	if err != nil {
		panic(err)
	}
	a.calls = append(a.calls, core.Call{
		Method:   method,
		Target:   common.HexToAddress(contractAddress),
		Name:     callName,
		CallData: callData,
	})
	return a
}

func (a *contract) AddMethod(signature string) ContractBuilder {
	existCall, ok := a.rawMethods[strings.ToLower(signature)]
	if ok {
		panic("Caller named " + existCall + " is exist on ABI")
	}
	a.rawMethods[strings.ToLower(signature)] = signature
	a.methods = append(a.methods, parseNewMethod(signature))
	newAbi, err := repackAbi(a.methods)
	if err != nil {
		panic(err)
	}
	a.contractAbi = newAbi
	if err != nil {
		panic(err)
	}
	return a
}

func (a *contract) Abi() abi.ABI {
	return a.contractAbi
}

func (a *contract) Call(blockNumber *big.Int) (*big.Int, map[string][]interface{}, error) {
	res := make(map[string][]interface{})
	blockNumber, results, err := a.multiCaller.Execute(a.calls, blockNumber)
	for _, call := range a.calls {
		res[call.Name], _ = a.contractAbi.Unpack(call.Method, results[call.Name].ReturnData)
	}
	a.ClearCall()
	return blockNumber, res, err
}

func (a *contract) ClearCall() {
	a.calls = []core.Call{}
}

func parseNewMethod(signature string) Method {
	methodPaths := strings.Split(signature, "(")
	if len(methodPaths) <= 1 {
		panic("Function is invalid format!")
	}
	methodName := strings.Replace(methodPaths[0], "function", "", 1)
	methodName = strings.TrimSpace(methodName)
	newMethod := Method{
		Name:            methodName,
		Inputs:          make([]Argument, 0),
		Outputs:         make([]Argument, 0),
		Type:            "function",
		StateMutability: "view",
	}

	isMultipleReturn := strings.Contains(signature, ")(")
	if isMultipleReturn {
		multipleReturnPaths := strings.Split(signature, ")(")
		multipleReturnPath := multipleReturnPaths[1]
		paramsPaths := strings.Split(multipleReturnPaths[0], "(")
		params := parseParamsPath(paramsPaths[1])
		if len(params) > 0 {
			for _, inParam := range params {
				if inParam != "" {
					newMethod.Inputs = append(newMethod.Inputs, Argument{
						Name:         "",
						Type:         strings.TrimSpace(inParam),
						InternalType: strings.TrimSpace(inParam),
					})
				}
			}
		}

		outputPath := strings.Replace(multipleReturnPath, ")", "", 1)
		outputs := strings.Split(outputPath, ",")

		for _, outParam := range outputs {
			newMethod.Outputs = append(newMethod.Outputs, Argument{
				Name:         "",
				Type:         strings.TrimSpace(outParam),
				InternalType: strings.TrimSpace(outParam),
			})
		}
	} else {
		singleReturnPaths := strings.Split(signature, ")")
		paramsPaths := strings.Split(singleReturnPaths[0], "(")
		params := parseParamsPath(paramsPaths[1])

		if len(params) > 0 {
			for _, inParam := range params {
				if inParam != "" {
					newMethod.Inputs = append(newMethod.Inputs, Argument{
						Name:         "",
						Type:         strings.TrimSpace(inParam),
						InternalType: strings.TrimSpace(inParam),
					})
				}
			}
		}

		returnType := strings.TrimSpace(singleReturnPaths[1])
		newMethod.Outputs = append(newMethod.Outputs, Argument{
			Name:         "",
			Type:         returnType,
			InternalType: returnType,
		})
	}
	return newMethod
}

func parseParamsPath(paramsPath string) []string {
	params := strings.Split(paramsPath, ",")
	return params
}

func repackAbi(methods []Method) (abi.ABI, error) {
	abiString, err := json.Marshal(methods)
	if err != nil {
		return abi.ABI{}, err
	}
	return abi.JSON(strings.NewReader(string(abiString)))
}
