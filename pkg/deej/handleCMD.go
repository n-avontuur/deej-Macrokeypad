package deej

import (
	"fmt"
	"strings"
	"time"
)

const (
	NORMAL_PRESS_DELAY = 500 * time.Millisecond // 0.50 seconds
	LONG_PRESS_DELAY   = 250 * time.Millisecond // 0.25 seconds
	SPAM_PRESS_DELAY   = 100 * time.Millisecond // 0.10 seconds

	NORMAL_PRESS = 200 * time.Millisecond  // 0.20 seconds
	LONG_PRESS   = 500 * time.Millisecond  // 0.50 seconds
	SPAM_PRESS   = 3000 * time.Millisecond // 3.00 seconds
)

var charToNum = map[rune]int{
	'0': 0,
	'1': 1,
	'2': 2,
	'3': 3,
	'4': 4,
	'5': 5,
	'6': 6,
	'7': 7,
	'8': 8,
	'9': 9,
	'/': 10,
	'=': 11,
	'*': 12,
}

var (
	FirstTimePressed  = time.Now()
	LastTimePressed   = time.Now()
	LastSpamPressTime = time.Now()
	LastLongPressTime = time.Now()
	FirstPress        = false
	pressWait         = 0 * time.Millisecond
	LastKeyValue      = 0
	ResultKeyValue    = 0
	ActivePage        = 0
)

func (deej *Deej) receiveKey(keyValue string) {
	if keyValue == "0" {
		return
	}
	ResultKeyValue = deej.handlingPressDuaration(keyValue)
	//fmt.Printf("receiveKey for handleCommands %d.\n", ResultKeyValue)
	deej.handleCommands(ResultKeyValue)
}

func convertStringToNumericalValues(input string) int {
	// Assuming we only need the numerical value of the first character
	if len(input) > 0 {
		if numValue, exists := charToNum[rune(input[0])]; exists {
			//fmt.Printf("The character '%c' translates to numerical value %d\n", input[0], numValue)
			return numValue
		}
	}
	//fmt.Printf("The character '%c' does not have a numerical value\n", input[0])
	return 0
}

func (deej *Deej) handlingPressDuaration(keyValue string) int {
	FirstPress = false
	currentTime := time.Now()
	numValue := convertStringToNumericalValues(keyValue)

	//fmt.Printf("Convert String To Numerical Values %d. \n", numValue)
	// reset all values after waiting for a serten amount
	if LastKeyValue != numValue {
		FirstTimePressed = currentTime
		FirstPress = true
		pressWait = 0
	} else {
		pressWait = currentTime.Sub(LastTimePressed)
	}

	LastTimePressed = currentTime
	LastKeyValue = numValue

	if pressWait > 100*time.Millisecond {
		FirstTimePressed = currentTime
		FirstPress = true
		pressWait = 0
	}

	pressDuration := currentTime.Sub(FirstTimePressed)

	delaytypemessage := ""
	if pressDuration >= SPAM_PRESS && currentTime.Sub(LastSpamPressTime) >= SPAM_PRESS_DELAY {
		LastSpamPressTime = currentTime
		ResultKeyValue = numValue
		delaytypemessage = " spam "
		fmt.Printf("Type of press %s and key active %d\n", delaytypemessage, ResultKeyValue)
		return ResultKeyValue
	}

	if pressDuration >= LONG_PRESS && currentTime.Sub(LastLongPressTime) >= LONG_PRESS_DELAY && pressDuration <= SPAM_PRESS {
		LastLongPressTime = currentTime
		ResultKeyValue = numValue
		delaytypemessage = " long "
		fmt.Printf("Type of press %s and key active %d\n", delaytypemessage, ResultKeyValue)
		return ResultKeyValue
	}

	if pressDuration < NORMAL_PRESS && FirstPress {
		ResultKeyValue = numValue
		delaytypemessage = " normal "
		fmt.Printf("Type of press %s and key active %d\n", delaytypemessage, ResultKeyValue)
		return ResultKeyValue
	}
	return 0
}

func (deej *Deej) handleCommands(key int) {
	//fmt.Printf("%+v\n. ", deej.config)
	if key == 0 {
		fmt.Println("Invalid key pressed value, possible 0")
		return
	}
	if ActivePage >= len(deej.config.key_commandos) {
		fmt.Println("Invalid active page")
		return
	}

	ActiveCommands, exists := deej.config.key_commandos[ActivePage].Commands[key]
	//fmt.Printf("Retreived active commando : %s \n", ActiveCommands.Commando)
	if !exists {
		fmt.Println("Command not found for key:", key)
		return
	}

	switch ActiveCommands.Type {
	case "Application":
		fmt.Printf("Application %s", ActiveCommands.Commando)

	case "assignFunctionToEncoder":
		fmt.Println(ActiveCommands.Commando)
		splitLine := strings.Split(ActiveCommands.Commando, " ")
		if len(splitLine) != 3 {
			fmt.Printf("Invalid command format lengt, len = %d. \n", len(splitLine))
			return
		}
		deej.assignFunctionToEncoder(strings.Split(splitLine[0], "-")[1], splitLine[1], splitLine[2])

	case "sendLine":
		//deej.serial.sendLine("625|1")

	case "TypeLetter4":
		fmt.Println("TypeLetter4")

	default:
		fmt.Println("Unknown command:", ActiveCommands.Commando)
	}
}

// func startApplication(v string) {
// 	s := []string{"cmd.exe", "/C", "start", v}

// 	cmd := exec.Command(s[0], s[1:]...)
// 	if err := cmd.Run(); err != nil {
// 		fmt.Println("Error:", err)
// 	}
// }

// func switchPage(pageNr int) {
// 	ActivePage = pageNr
// }
