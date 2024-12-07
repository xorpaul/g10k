package logging

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

var validationMessages []string
var Debug bool
var Verbose bool
var Info bool
var Validate bool
var Quiet bool

// Debugf is a helper function for debug logging if global variable debug is set to true
func Debugf(s string) {
	if Debug {
		pc, _, _, _ := runtime.Caller(1)
		callingFunctionName := strings.Split(runtime.FuncForPC(pc).Name(), ".")[len(strings.Split(runtime.FuncForPC(pc).Name(), "."))-1]
		if strings.HasPrefix(callingFunctionName, "func") {
			// check for anonymous function names
			log.Print("DEBUG " + fmt.Sprint(s))
		} else {
			log.Print("DEBUG " + callingFunctionName + "(): " + fmt.Sprint(s))
		}
	}
}

// Verbosef is a helper function for verbose logging if global variable verbose is set to true
func Verbosef(s string) {
	if Debug || Verbose {
		log.Print(fmt.Sprint(s))
	}
}

// Infof is a helper function for info logging if global variable info is set to true
func Infof(s string) {
	if Debug || Verbose || Info {
		color.Green(s)
	}
}

// Validatef is a helper function for validation logging if global variable validate is set to true
func Validatef() {
	if len(validationMessages) > 0 {
		for _, message := range validationMessages {
			color.New(color.FgRed).Fprintln(os.Stdout, message)
		}
		os.Exit(1)
	} else {
		color.New(color.FgGreen).Fprintln(os.Stdout, "Configuration successfully parsed.")
		os.Exit(0)
	}
}

// Warnf is a helper function for warning logging
func Warnf(s string) {
	color.Set(color.FgYellow)
	fmt.Println(s)
	color.Unset()
}

// Fatalf is a helper function for fatal logging
func Fatalf(s string) {
	if Validate {
		validationMessages = append(validationMessages, s)
	} else {
		color.New(color.FgRed).Fprintln(os.Stderr, s)
		os.Exit(1)
	}
}

// funcName return the function name as a string
func FuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	completeFuncname := runtime.FuncForPC(pc).Name()
	return strings.Split(completeFuncname, ".")[len(strings.Split(completeFuncname, "."))-1]
}
