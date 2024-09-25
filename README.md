# chick - Enhanced DNS and I-line Checker

chick is a command-line tool that provides enhanced DNS lookup functionality. It checks PTR records for IP addresses, fetches country and organization information using the ipinfo.io API, and retrieves I-line information for each IP address using the IRCnet API.

## Features

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

## Installation

### Prerequisites

- Go 1.16 or higher

### Building from source

1. Clone the repository:
git clone https://github.com/kofany/chick.git
cd chick
Copy
2. Install dependencies:
go get github.com/alecthomas/kong
go get github.com/fatih/color
Copy
3. Build the program:
go build -o chick
Copy
## Usage
./chick [options] <domain/ip>
Copy
### Options

- `-4`: Show only IPv4 (A) records
- `-6`: Show only IPv6 (AAAA) records
- `--timeout`: Set timeout for HTTP requests (default: 5s)
- `--iline-timeout`: Set timeout for I-line API requests (default: 10s)
- `-h, --help`: Show help message

### Examples
./chick example.com
./chick -4 example.com
./chick -6 2001:db8::1
./chick --timeout 10s 8.8.8.8
Copy
## Output

For each IP address, chick displays:

- A or AAAA record
- PTR records (if available)
- Country and Organization (from ipinfo.io)
- I-Line servers (from IRCnet API)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](https://kofany.mit-license.org) file for details.

## Acknowledgments

- [ipinfo.io](https://ipinfo.io/) for providing IP information API
- [IRCnet API](https://bot.ircnet.info/api/) for providing I-line information
- [fatih/color](https://github.com/fatih/color) for colorized console output
- [alecthomas/kong](https://github.com/alecthomas/kong) for CLI argument parsing and help generation

## Author

Jerzy "kofany" Dąbrowski
