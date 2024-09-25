/*
Extended DNS Check (chick) is a command-line tool that provides enhanced DNS lookup functionality.
It checks PTR records for IP addresses and fetches additional IP information using the ipinfo.io API.
For domains or subdomains, it displays A and AAAA records and retrieves IP information for each resolved address.
It also fetches I-line information for each IP address using the IRCnet API.

Features:
  - Lookup A and AAAA records for domains and subdomains
  - Retrieve PTR records for IP addresses
  - Fetch country and organization information using ipinfo.io API
  - Fetch I-line information using IRCnet API
  - Support for IPv4 and IPv6 addresses
  - Colorized output for better readability
  - Parallel processing using goroutines
  - Configurable timeout handling for HTTP requests
  - Graceful shutdown on user interrupt
  - Progress information during execution

GitHub Repository: https://github.com/kofany/chick

Author: Jerzy Dąbrowski

License: MIT License (https://kofany.mit-license.org)
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
)

var CLI struct {
	IPv4         bool          `help:"Show only IPv4 (A) records" short:"4"`
	IPv6         bool          `help:"Show only IPv6 (AAAA) records" short:"6"`
	Timeout      time.Duration `help:"Timeout for HTTP requests" default:"5s"`
	ILineTimeout time.Duration `help:"Timeout for I-line API requests" default:"10s"`
	Target       string        `arg name:"domain/ip" help:"Domain, subdomain or IP to check"`
}

type IPInfo struct {
	IP          string `json:"ip"`
	Country     string `json:"country"`
	Org         string `json:"org"`
}

type ILineInfo struct {
	Status   string `json:"status"`
	Response []struct {
		ServerName string `json:"serverName"`
	} `json:"response"`
}

type Result struct {
	IP     string
	PTR    []string
	IPInfo *IPInfo
	ILine  []string
	IsIPv6 bool
	Error  error
}

var (
	cyan    = color.New(color.FgCyan).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	green   = color.New(color.FgGreen).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
)

var httpClient *http.Client

func getIPInfo(ctx context.Context, ip string) (*IPInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://ipinfo.io/"+ip+"/json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ipInfo IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&ipInfo); err != nil {
		return nil, err
	}
	return &ipInfo, nil
}

func getILineInfo(ctx context.Context, ip string) ([]string, error) {
	url := "https://bot.ircnet.info/api/i-line?q=" + ip
	
	ilineCtx, cancel := context.WithTimeout(ctx, CLI.ILineTimeout)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ilineCtx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	ilineClient := &http.Client{Timeout: CLI.ILineTimeout}
	resp, err := ilineClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var iLineInfo ILineInfo
	if err := json.Unmarshal(body, &iLineInfo); err != nil {
		return nil, err
	}

	if iLineInfo.Status != "SUCCESS" {
		return nil, fmt.Errorf("failed to get I-line info")
	}

	var servers []string
	for _, server := range iLineInfo.Response {
		servers = append(servers, server.ServerName)
	}

	return servers, nil
}

func lookupIP(ctx context.Context, ip string, isIPv6 bool, resultChan chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()
	result := Result{IP: ip, IsIPv6: isIPv6}

	var wgInternal sync.WaitGroup
	wgInternal.Add(3)

	go func() {
		defer wgInternal.Done()
		names, err := net.LookupAddr(ip)
		if err != nil {
			result.Error = fmt.Errorf("error looking up PTR records: %v", err)
		} else {
			result.PTR = names
		}
	}()

	go func() {
		defer wgInternal.Done()
		ipInfo, err := getIPInfo(ctx, ip)
		if err != nil {
			if result.Error != nil {
				result.Error = fmt.Errorf("%v; error fetching IP info: %v", result.Error, err)
			} else {
				result.Error = fmt.Errorf("error fetching IP info: %v", err)
			}
		} else {
			result.IPInfo = ipInfo
		}
	}()

	go func() {
		defer wgInternal.Done()
		iLine, err := getILineInfo(ctx, ip)
		if err != nil {
			if result.Error != nil {
				result.Error = fmt.Errorf("%v; error fetching I-line info: %v", result.Error, err)
			} else {
				result.Error = fmt.Errorf("error fetching I-line info: %v", err)
			}
		} else {
			result.ILine = iLine
		}
	}()

	wgInternal.Wait()
	resultChan <- result
}

func printResult(result Result) {
	recordType := "A"
	if result.IsIPv6 {
		recordType = "AAAA"
	}
	fmt.Printf("%s: %s\n", cyan(fmt.Sprintf("%s Record", recordType)), yellow(result.IP))

	if len(result.PTR) > 0 {
		fmt.Printf("  %s: %s\n", cyan("PTR Records"), green(strings.Join(result.PTR, ", ")))
	}

	if result.IPInfo != nil {
		fmt.Printf("  %s: %s\n", cyan("Country"), green(result.IPInfo.Country))
		fmt.Printf("  %s: %s\n", cyan("Organization"), green(result.IPInfo.Org))
	}

	if len(result.ILine) > 0 {
		fmt.Printf("  %s: %s\n", cyan("I-Line Servers"), green(strings.Join(result.ILine, ", ")))
	}

	if result.Error != nil {
		fmt.Printf("  %s: %s\n", red("Error"), red(result.Error.Error()))
	}

	fmt.Println()
}

func validateInput(input string) error {
	if net.ParseIP(input) != nil {
		return nil
	}
	if _, err := net.LookupHost(input); err != nil {
		return fmt.Errorf("invalid domain or IP address: %v", err)
	}
	return nil
}

func main() {
	ctx := kong.Parse(&CLI)

	if err := validateInput(CLI.Target); err != nil {
		fmt.Printf("%s: %v\n", red("Error"), red(err))
		ctx.Exit(1)
	}

	httpClient = &http.Client{Timeout: CLI.Timeout}

	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupt received, shutting down...")
		cancel()
	}()

	ip := net.ParseIP(CLI.Target)

	var wg sync.WaitGroup
	resultChan := make(chan Result, 10) // Buffered channel
	var results []Result

	var ips []net.IP
	if ip != nil {
		ips = append(ips, ip)
	} else {
		var err error
		ips, err = net.LookupIP(CLI.Target)
		if err != nil {
			fmt.Printf("%s: %v\n", red("Error looking up IP for domain"), red(err))
			ctx.Exit(1)
		}
	}

	totalIPs := 0
	for _, ip := range ips {
		isIPv6 := ip.To4() == nil
		if (CLI.IPv4 && !isIPv6) || (CLI.IPv6 && isIPv6) || (!CLI.IPv4 && !CLI.IPv6) {
			totalIPs++
			wg.Add(1)
			go lookupIP(mainCtx, ip.String(), isIPv6, resultChan, &wg)
		}
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	done := make(chan bool)
	go func() {
		for result := range resultChan {
			results = append(results, result)
		}
		close(done)
	}()

	fmt.Print(yellow("Checking records... Please wait"))
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	processed := 0
	for {
		select {
		case <-mainCtx.Done():
			fmt.Println("\nOperation cancelled")
			return
		case <-done:
			fmt.Print("\r" + strings.Repeat(" ", 60) + "\r") // Clear the progress message
			for _, result := range results {
				printResult(result)
			}
			return
		case <-ticker.C:
			processed = len(results)
			fmt.Printf("\rChecking records... %d/%d completed", processed, totalIPs)
		}
	}
}
