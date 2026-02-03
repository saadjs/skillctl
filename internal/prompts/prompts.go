package prompts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var errCanceled = errors.New("canceled")

func ErrCanceled() error {
	return errCanceled
}

func AskInput(label string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s: ", label)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func AskYesNo(label string, defaultYes bool) (bool, error) {
	def := "y/N"
	if defaultYes {
		def = "Y/n"
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [%s]: ", label, def)
		text, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		value := strings.TrimSpace(strings.ToLower(text))
		if value == "" {
			return defaultYes, nil
		}
		switch value {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		case "q", "quit":
			return false, errCanceled
		}
		fmt.Println("Please enter y or n (or q to quit).")
	}
}

func AskSelect(label string, options []string) (string, error) {
	if len(options) == 0 {
		return "", errors.New("no options")
	}
	fmt.Println(label)
	for i, option := range options {
		fmt.Printf("%d) %s\n", i+1, option)
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Select [1-%d] (or q to quit): ", len(options))
		text, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(strings.ToLower(text))
		if value == "q" || value == "quit" {
			return "", errCanceled
		}
		idx, err := strconv.Atoi(value)
		if err != nil || idx < 1 || idx > len(options) {
			fmt.Println("Invalid selection.")
			continue
		}
		return options[idx-1], nil
	}
}

func AskMulti(label string, options []string) ([]string, error) {
	if len(options) == 0 {
		return nil, errors.New("no options")
	}
	fmt.Println(label)
	for i, option := range options {
		fmt.Printf("%d) %s\n", i+1, option)
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Select one or more numbers (comma-separated), or q to quit: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		value := strings.TrimSpace(strings.ToLower(text))
		if value == "q" || value == "quit" {
			return nil, errCanceled
		}
		if value == "" {
			fmt.Println("Please enter at least one selection.")
			continue
		}
		parts := strings.Split(value, ",")
		var selections []string
		seen := map[int]bool{}
		valid := true
		for _, part := range parts {
			part = strings.TrimSpace(part)
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 1 || idx > len(options) {
				valid = false
				break
			}
			if !seen[idx] {
				selections = append(selections, options[idx-1])
				seen[idx] = true
			}
		}
		if !valid {
			fmt.Println("Invalid selection.")
			continue
		}
		return selections, nil
	}
}
