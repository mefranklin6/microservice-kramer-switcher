package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Dartmouth-OpenAV/microservice-framework/framework"
	"github.com/fatih/color"
)

const NAME_OUTPUT_ADD = 1000 // An audio channel name with this added means Kramer output stage
var maxRecur = 5
var recursions = 0

// Send a command to the device and then read the response from it.
func sendCommand(socketKey string, command []byte, expectedResponse string) (string, error) {
	function := "sendCommand"
	// framework.Log(function + " - Command: " + string(command) + " for: " + socketKey)

	// setLineDelimiter(defaultLineDelimiter) // handled in microservice.go setFrameworkGlobals()

	// First read past any pending output from the Kramer (seems to happen for multiple reasons)
	readPast := "start"
	for len(readPast) != 0 {
		readPast = framework.ReadLineFromSocket(socketKey)
		if len(readPast) != 0 {
			framework.Log(fmt.Sprintf("RRRRR Read past pending data from Kramer got: [%s]", readPast))
			readPast = framework.ReadLineFromSocket(socketKey)
		}
	}

	// Write our command
	line := string(command) + string(0xD) + string(0xA) // Kramer needs CRLF after the command
	if !framework.WriteLineToSocket(socketKey, line) {
		// the socket connection may have died, so we'll try once to reopen it
		errMsg := function + " - error writing to " + socketKey + " closing and trying to open the socket again"
		framework.AddToErrors(socketKey, errMsg)
		framework.CloseSocketConnection(socketKey)

		if !framework.WriteLineToSocket(socketKey, line) {
			errMsg := function + " - still getting an error writing to " + socketKey + " giving up"
			framework.AddToErrors(socketKey, errMsg)
			framework.CloseSocketConnection(socketKey)
			return string(command), errors.New(errMsg)
		}
	}

	// Now we need to read the response
	var retMsg string
	retMsg = "uninitialized"
	msg := framework.ReadLineFromSocket(socketKey)
	// Check for and read any more responses (we'll use the last)
	nextMsg := "start"
	for len(nextMsg) != 0 {
		nextMsg = framework.ReadLineFromSocket(socketKey)
		if len(nextMsg) != 0 {
			nextRespVals := strings.Split(string(nextMsg), " ") // Kramer responses are space delimited
			if len(nextRespVals) > 0 {
				if strings.Contains(nextRespVals[0], expectedResponse) { // a later response matches what we expected, so let's use it
					framework.Log("LLLLL Got an expected response on a later read:" + nextMsg)
					msg = nextMsg
				} else {
					framework.Log("LLLLL Got an UNexpected response on a later read:" + nextMsg)
				}
			}
		}
	}
	origMsg := msg
	if msg == "" { // No response
		if recursions < maxRecur { // haven't exhausted our available time
			recursions++
			// close socket and try again (Kramer VP-440H2 appears to need this once in a while)
			framework.Log(function + " - recurring")
			framework.CloseSocketConnection(socketKey)
			recur_msg, recur_err := sendCommand(socketKey, command, expectedResponse)
			recursions--
			return recur_msg, recur_err
		}
		errMsg := function + " - asdfasf2323 error reading from " + socketKey
		framework.AddToErrors(socketKey, errMsg)
		framework.CloseSocketConnection(socketKey)
		return string(command), errors.New(errMsg)
	} else { // Got something to process
		// Process what we read
		if len(msg) > 3 {
			// check for leading tilde - a sign of healthy communication
			if msg[0] != '~' {
				errMsg := function + " - error didn't find a ~ in response from " + socketKey
				framework.AddToErrors(socketKey, errMsg)
				return string(command), errors.New(errMsg)
			}
			// truncate the bytes slice to get rid of the CRLF that Kramer returns
			msg = msg[:len(msg)-2]
			// truncate the bytes slice to get rid of the ~address (usually ~01) in the response
			msg = msg[4:len(msg)]
			//framework.Log(fmt.Sprintf("**** processed msg is: %s\n", string(msg)))
		}
		//framework.Log(fmt.Sprintf("Command [%s] got actual response [%s]\n", string(command), msg))
		stringMsg := string(msg)
		respVals := strings.Split(stringMsg, " ") // Kramer responses are space delimited

		if !strings.Contains(string(command), "?") { // If it's a set command
			//framework.Log(fmt.Sprintf("Set command: %s respVals[2]: %s\n", string(command), respVals[2]))
			if len(respVals) < 2 { // Used to check for "OK" responses, but VP-558 doesn't always emit "OK" on success :-/
				errMsg := fmt.Sprintf(function+" - hjjkn87 got too few tokens in response from %s to a set command. The command was: %s response was: %s\n\n",
					socketKey, string(command), string(origMsg))
				framework.Log(errMsg)
			}
		}
		//framework.Log(fmt.Sprintf("respVals[0]: %s, expectedResponse: %s", respVals[0], expectedResponse))
		retMsg = origMsg
		if respVals[0] == expectedResponse {
			// Since they match this was presumably responding to our command and we'll use the most recent that matched
			// framework.Log(fmt.Sprintf("Got expected response: [%s], retMsg is: [%s]\n", expectedResponse, retMsg))
		} else { // Didn't get expected response, so we'll loop again in case we can read past extraneous responses and find the one we want
			errMsg := fmt.Sprintf(function+" - vads#432 Didn't get expected response from %s. The command was: %s response was: %s\n\n",
				socketKey, string(command), string(origMsg))
			framework.Log(errMsg)
		}
	}

	//global_recur_level = global_recur_level - 1
	//framework.Log(fmt.Sprintf(color.HiBlueString("sendCommand() Done sending command [%s] to %v, RECUR level = %d", string(command), address, global_recur_level)))
	framework.Log(fmt.Sprintf(function+" - Success: got expected response: [%s], retMsg is: [%s]\n", expectedResponse, retMsg))
	return retMsg, nil
}

// Implements the hack to allow Kramer names >= NAME_OUTPUT_ADD (1000) so we can reuse the volume routines
//
//	for both input and output stage
func convertName(name string) (string, string) {
	intname, err := strconv.Atoi(name)
	if err != nil {
		framework.Log(fmt.Sprintf("Non-integer input/output name: %s, using 1 on input stage", name))
		return "0", "1"
	}
	stage := "undefined"
	if intname >= NAME_OUTPUT_ADD {
		stage = "1" // Output
		intname = intname - NAME_OUTPUT_ADD
	} else {
		stage = "0" // Input
	}
	nameStr := strconv.Itoa(intname)
	return stage, nameStr
}

func setAudioMute(socketKey string, name string, value string) (string, error) {
	theModel, _ := getModel(socketKey)
	if strings.Contains(theModel, "VS-88UT") || strings.Contains(theModel, "VS-84UT") {
		return vsDoSetAudioMute(socketKey, name, value)
	} else {
		return doSetAudioMute(socketKey, name, value)
	}
}

func doSetAudioMute(socketKey string, name string, value string) (string, error) {
	function := "doSetAudioMute"

	onoff := "unknown"

	if value == `"true"` {
		onoff = "1" // Values for video disable in Kramer VMUTE command
	} else if value == `"false"` {
		onoff = "0" // Enable video
	} else {
		errMsg := fmt.Sprintf(function + " - unrecognized audiomute value: " + value)
		framework.AddToErrors(socketKey, errMsg)
		return value, errors.New(errMsg)
	}

	baseStr := "MUTE"
	commandStr := []byte("#" + baseStr + " " + name + "," + onoff)
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	//framework.Log(fmt.Sprintf(function + " - ot resp: " + string(resp) + "\n")
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error sending command %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	// Parse the value we got back
	respVals := strings.Split(string(resp), " ") // Kramer delimits responses with spaces
	respMute := strings.Split(respVals[1], ",")  // Kramer delimits the mute part of the response with commas
	if len(respMute) != 2 {
		errMsg := fmt.Sprintf(function+" - fsar332 wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	if respMute[1] == "0" {
		value = `"false"`
	} else if respMute[1] == "1" {
		value = `"true"`
	} else { // not a legal value
		errMsg := fmt.Sprintf(function + " - unrecognized response to mute command: " + respMute[1])
		framework.Log(errMsg)
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	// If we got here, the response was good, so successful return with the state indication
	return value, nil
}

func getAudioMute(socketKey string, name string) (string, error) {
	theModel, _ := getModel(socketKey)
	if strings.Contains(theModel, "VS-88UT") || strings.Contains(theModel, "VS-84UT") {
		return vsDoGetAudioMute(socketKey, name)
	} else {
		return doGetAudioMute(socketKey, name)
	}
}

func doGetAudioMute(socketKey string, name string) (string, error) {
	function := "doGetAudioMute"

	baseStr := "MUTE"
	commandStr := []byte("#" + baseStr + "? " + name)
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	//framework.Log(fmt.Sprintf("Got resp: " + string(resp) + "\n"))
	if err != nil {
		framework.Log(fmt.Sprintf(color.HiRedString("Error 21c: %v", err.Error())))
		return resp, err
	}
	// Parse the value we got back
	respVals := strings.Split(string(resp), " ") // Kramer delimits responses with spaces
	respMute := strings.Split(respVals[1], ",")  // Kramer delimits the mute part of the response with commas
	if len(respMute) != 2 {
		errMsg := fmt.Sprintf(function+" - fmcx3rsd wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	value := "unknown"
	if respMute[1] == "1" {
		value = "true"
	} else if respMute[1] == "0" {
		value = "false"
	} else { // not a legal value
		errMsg := function + " - unrecognized audio mute returned: " + respMute[1] + " is not a legal value\n"
		framework.AddToErrors(socketKey, errMsg)
		return value, errors.New(errMsg)

	}
	// If we got here, the response was good, so successful return with the state indication
	return `"` + value + `"`, nil
}

func setVideoMute(socketKey, output string, value string) (string, error) {
	theModel, _ := getModel(socketKey)
	if strings.Contains(theModel, "VS-88UT") || strings.Contains(theModel, "VS-84UT") {
		return vsDoSetVideoMute(socketKey, output, value)
	} else {
		return doSetVideoMute(socketKey, output, value)
	}
}

func doSetVideoMute(socketKey, output string, value string) (string, error) {
	function := "setVideoMute"
	onoff := "not set"

	// VP-558 has a bug that they reversed video mute logic, so we need to find out what model we have and
	//  act differently for VP-558
	theModel, _ := getModel(socketKey)

	if value == `"true"` {
		onoff = "1" // Values for video disable in Kramer VMUTE command
	} else if value == `"false"` {
		onoff = "0" // Enable video
	} else if value == `"blank"` {
		onoff = "2" // Blank the image
	} else {
		errMsg := fmt.Sprintf(function + " - unrecognized videomute value: " + value)
		framework.AddToErrors(socketKey, errMsg)
		return value, errors.New(errMsg)
	}

	// Now reverse the logic for VP-558
	if theModel == `"VP-558"` {
		if onoff == "1" {
			onoff = "0"
		} else if onoff == "0" {
			onoff = "1"
		}
	}

	baseStr := "VMUTE"
	commandStr := []byte("#" + baseStr + " " + output + "," + onoff)
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	//framework.Log(fmt.Sprintf("Got resp: " + string(resp) + "\n")
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error sending command %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	// Parse the value we got back
	respVals := strings.Split(string(resp), " ") // Kramer delimits responses with spaces
	respMute := strings.Split(respVals[1], ",")  // Kramer delimits the mute part of the response with commas
	if len(respMute) != 2 {
		errMsg := fmt.Sprintf(function+" - bbfsd4 wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	if respMute[1] == "0" {
		value = "false"
	} else if respMute[1] == "1" {
		value = "true"
	} else if respMute[1] == "2" {
		value = "blank"
	} else { // not a legal value
		errMsg := fmt.Sprintf(function + " - unrecognized response to mute command: " + respMute[1])
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	// Now reverse the logic for VP-558
	if theModel == `"VP-558"` {
		if value == "true" {
			value = "false"
		} else if value == "false" {
			value = "true"
		}
	}

	// If we got here, the response was good, so successful return with the state indication

	return value, nil
}

func getVideoMute(socketKey, output string) (string, error) {
	theModel, _ := getModel(socketKey)
	if strings.Contains(theModel, "VS-88UT") || strings.Contains(theModel, "VS-84UT") {
		return vsDoGetVideoMute(socketKey, output)
	} else {
		return doGetVideoMute(socketKey, output)
	}
}

func doGetVideoMute(socketKey string, output string) (string, error) {
	function := "getVideoMute"

	// VP-558 has a bug that they reversed video mute logic, so we need to find out what model we have and
	//  act differently for VP-558
	theModel, _ := getModel(socketKey)

	baseStr := "VMUTE"
	commandStr := []byte("#" + baseStr + "? " + output)
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	framework.Log(fmt.Sprintf(function + " - got resp: " + string(resp) + "\n"))
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error sending command %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	// Parse the value we got back
	respVals := strings.Split(string(resp), " ") // Kramer delimits responses with spaces
	respMute := strings.Split(respVals[1], ",")  // Kramer delimits the mute part of the response with commas
	if len(respMute) != 2 {
		errMsg := fmt.Sprintf(function+" - 54fsav wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	value := "unknown" // Just to appease go syntax
	if respMute[1] == "0" {
		value = "false"
	} else if respMute[1] == "1" {
		value = "true"
	} else if respMute[1] == "2" {
		value = "blank"
	} else { // not a legal value
		errMsg := function + " - get video mute returned: " + respMute[1] + " which is not a legal value\n"
		framework.AddToErrors(socketKey, errMsg)
		return errMsg, errors.New(errMsg)
	}

	// Now reverse the logic for VP-558
	if theModel == `"VP-558"` {
		if value == "true" {
			value = "false"
		} else if value == "false" {
			value = "true"
		}
	}
	framework.Log(function + " - theModel: " + theModel + " value: " + value)

	return `"` + value + `"`, nil
}

func setVolume(socketKey string, name string, level string) (string, error) {
	function := "setVolume"
	baseStr := "AUD-LVL"
	level = strings.Replace(level, `"`, "", -1) // Get rid of the JSON body quotes
	stage, realName := convertName(name)
	commandStr := []byte("#" + baseStr + " " + stage + "," + realName + "," + level) // We only manipulate and report audio on input (stage 0)
	//framework.Log(fmt.Sprintf(function + " - [%s][%s]\n", name, string(commandStr)))
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	//framework.Log(fmt.Sprintf(function + " - Got resp: " + string(resp) + "\n"))
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error sending command %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	// Verify the value we got back
	respVals := strings.Split(string(resp), " ") // Kramer delimits responses with spaces
	respLevel := strings.Split(respVals[1], ",") // Kramer delimits the volume part of the response with commas
	if len(respLevel) != 3 {
		errMsg := fmt.Sprintf(function+" - bcdgfs34 wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	if respLevel[2] != level {
		framework.Log(fmt.Sprintf(function+" - Error n3qwerw: level returned: %d doesn't match level set: %d\n", respLevel[2], level))
	}
	//framework.Log(fmt.Sprintf("XXX DoSetVolume returning: %d\n", level))
	return level, nil // Effectively returning what was passed to us - is this right?
}

func getVolume(socketKey string, name string) (string, error) {
	function := "getVolume"

	baseStr := "AUD-LVL"
	stage, realName := convertName(name)
	commandStr := []byte("#" + baseStr + "? " + stage + "," + realName)
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	// framework.Log(fmt.Sprintf(function + " - Got resp: " + string(resp) + "\n"))
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error sending command %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	// Find out what value we got
	respVals := strings.Split(string(resp), " ") // Kramer delimits responses with spaces
	respLevel := strings.Split(respVals[1], ",") // Kramer delimits the volume part of the response with commas
	if len(respLevel) != 3 {
		errMsg := fmt.Sprintf(function+" - vasa2qesd wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	// framework.Log(fmt.Sprintf("respVals: [%s][%s][%s]", respVals[0], respVals[1], respVals[2]))
	respGain := respLevel[2] // value is after the second comma, base 0 array
	respGainInt, errAtoi := strconv.Atoi(respGain)
	respGainInt = respGainInt // Appease golang
	if errAtoi != nil {
		errMsg := fmt.Sprintf(function+" - Invalid volume value in device response received: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	return `"` + respGain + `"`, nil
}

func getModel(socketKey string) (string, error) {
	function := "getModel"
	modelEndPoint := "model"

	var model string
	if !framework.CheckForEndPointInCache(socketKey, modelEndPoint) {
		baseStr := "MODEL"
		commandStr := []byte("#" + baseStr + "?")
		resp, err := sendCommand(socketKey, commandStr, baseStr)
		framework.Log(fmt.Sprintf(function + " - Got resp: " + string(resp) + "\n"))
		if err != nil {
			errMsg := fmt.Sprintf(function+" - error sending #MODEL command %v", err.Error())
			framework.AddToErrors(socketKey, errMsg)
			return string(resp), errors.New(errMsg)
		}
		respVals := strings.Split(resp, " ")
		if len(respVals) != 2 {
			errMsg := fmt.Sprintf(function + " - wrong number of tokens in model response")
			framework.AddToErrors(socketKey, errMsg)
			return string(resp), errors.New(errMsg)
		}
		model = respVals[1]

		framework.Log(function + " - saving model to deviceStates: [" + model + "]")
		framework.SetDeviceStateEndpoint(socketKey, modelEndPoint, model)
	} else {
		model = framework.GetDeviceStateEndpoint(socketKey, modelEndPoint)
	}

	theModel := `"` + model + `"`
	framework.Log(function + " - returning: " + theModel)
	return theModel, nil
}

func setVideoRoute(socketKey string, output string, input string) (string, error) {
	theModel, _ := getModel(socketKey)
	if strings.Contains(theModel, "VS-88UT") || strings.Contains(theModel, "VS-84UT") {
		return vsDoSetVideoRoute(socketKey, output, input)
	} else {
		return doSetVideoRoute(socketKey, output, input, "onlyVideo")
	}
}

func setAudioAndVideoRoute(socketKey string, output string, input string) (string, error) {
	return doSetVideoRoute(socketKey, output, input, "audioAndVideo")
}

func doSetVideoRoute(socketKey string, output string, input string, mode string) (string, error) {
	function := "doSetVideoRoute"
	baseStr := "ROUTE" // Kramer command that returns input information
	noQuotesInput := strings.Replace(input, `"`, "", -1)
	layer := ""
	if mode == "onlyVideo" {
		layer = " 1,"
	} else if mode == "audioAndVideo" {
		layer = " 12,"
	}

	commandStr := []byte("#" + baseStr + layer + output + "," + noQuotesInput)
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error sending command %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	// Now we need to parse the response from the Kramer
	//framework.Log(fmt.Sprintf("doSetInput resp: [%s]\n", resp))

	// Parse and verify the value we got back
	respVals := strings.Split(string(resp), " ") // Kramer delimits responses with spaces
	respLevel := strings.Split(respVals[1], ",") // Kramer delimits the video route part of the response with commas
	if len(respLevel) != 3 {
		errMsg := fmt.Sprintf(function+" - 445dfc wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	if respLevel[1] != output {
		errMsg := fmt.Sprintf(function+" - output returned: %s doesn't match output specified: %s\n", respLevel[1], output)
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	if `"`+respLevel[2]+`"` != input { // add quotes because the input and the cache store values in JSON
		errMsg := fmt.Sprintf(function+" - input returned: %s doesn't match input set: %s\n", `"`+respLevel[2]+`"`, input)
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	returnStr := `"` + input + `"`
	return returnStr, nil
}

func getVideoRoute(socketKey string, output string) (string, error) {
	theModel, _ := getModel(socketKey)
	if strings.Contains(theModel, "VS-88UT") || strings.Contains(theModel, "VS-84UT") {
		return vsDoGetVideoRoute(socketKey, output)
	} else {
		return doGetVideoRoute(socketKey, output)
	}
}

func getAudioAndVideoRoute(socketKey string, output string) (string, error) {
	return doGetVideoRoute(socketKey, output)
}

func doGetVideoRoute(socketKey string, output string) (string, error) {
	function := "doGetVideoRoute"
	framework.Log(function)
	baseStr := "ROUTE" // Kramer command that returns input information
	layer := " 1,"     // Always get VideoRoute (layer=1) because the Kramer device throws error when getting AudioAndVideo route (layer=12)
	commandStr := []byte("#" + baseStr + "?" + layer + output)
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error sending command %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	respVals := strings.Split(string(resp), " ") // Kramer delimits responses with spaces
	respRoute := strings.Split(respVals[1], ",") // Kramer delimits the volume part of the response with commas
	if len(respRoute) != 3 {
		errMsg := fmt.Sprintf(function+" - sd23q4gd wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	intOut, err := strconv.Atoi(string(respRoute[1]))
	intOut = intOut // appease
	intIn, err2 := strconv.Atoi(string(respRoute[2]))
	if err != nil || err2 != nil { // Illegal input or output
		errMsg := function + " - Invalid input or output value in device response received: " +
			string(respRoute[1]) + " " + string(respRoute[2])
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	// VP-778 has an off-by-one bug.  Fix it by adding 1 to the input channel number on get.
	theModel, _ := getModel(socketKey)
	if theModel == `"VP-778"` {
		intIn = intIn + 1
	}
	framework.Log(function + " - theModel: " + theModel)

	returnStr := `"` + strconv.Itoa(intIn) + `"`
	framework.Log(function + " - returning " + returnStr)
	return returnStr, nil
}

func setAudioRoute(socketKey string, output string, input string) (string, error) {
	return vsDoSetAudioRoute(socketKey, output, input)
}

func getAudioRoute(socketKey string, output string) (string, error) {
	return vsDoGetAudioRoute(socketKey, output)
}

func getOccupancyStatus(socketKey string, input string) (string, error) {
	function := "getOccupancyStatus"
	resp, err := getVideoInputStatus(socketKey, input)
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error returned by getVideoInputStatus %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	if resp == "Connected" {
		return `{"occupancy_detected":"true"}`, nil
	} else {
		return `{"occupancy_detected":"false"}`, nil
	}
}

// See if there is a signal connected to the specified input
// func doGetInputStatus(address string, input string) (status.ActiveInput, error) {
func getVideoInputStatus(socketKey string, input string) (string, error) {
	function := "getInputStatus"
	baseStr := "SIGNAL" // Kramer command that returns input connection status information
	commandStr := []byte("#" + baseStr + "? " + input)
	resp, err := sendCommand(socketKey, commandStr, baseStr)
	if err != nil {
		errMsg := fmt.Sprintf(function+" - error sending command %v", err.Error())
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	respVals := strings.Split(string(resp), " ")       // Kramer delimits responses with spaces
	respInputStatus := strings.Split(respVals[1], ",") // Kramer delimits the volume part of the response with commas
	if len(respInputStatus) != 2 {
		errMsg := fmt.Sprintf(function+" - hkj456dg wrong number of tokens in response: %s", string(resp))
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}

	intIn, err := strconv.Atoi(string(respInputStatus[0]))
	intIn = intIn // appease the golang parser
	intStatus, err2 := strconv.Atoi(string(respInputStatus[1]))
	if err != nil || err2 != nil { // Illegal input or output
		// return status.ActiveInput{"failed to get status"}, errors.New("status returned was invalid")
		errMsg := function + " - Invalid input or output value in device response received: " +
			string(respInputStatus[1]) + " " + string(respInputStatus[2])
		framework.AddToErrors(socketKey, errMsg)
		return string(resp), errors.New(errMsg)
	}
	returnStr := "true"
	if intStatus == 0 {
		returnStr = "false"
	}
	return `"` + returnStr + `"`, nil
}

func healthCheck(socketKey string) (string, error) {
	resp, err := getModel(socketKey)
	returnStr := "true"
	if err != nil {
		returnStr = "false"
	} else {
		returnStr = returnStr + " model: " + strings.Trim(resp, `"`)
	}
	return `"` + returnStr + `"`, nil
}
