<p align="center">
<img height="200" src="static/logo.png">
<br> IP To CloudProvider
</p>


IP To CloudProvider is a command-line tool to manage and check IP ranges for various service providers. It allows you to update IP ranges for specific providers, check if an IP belongs to any provider's range, and even verify a list of IPs from a file.

## Features

- **Update IP Ranges:** Keep the IP ranges for various service providers up-to-date with a single command.
- **Check IP:** Determine if a specific IP belongs to the range of any supported provider.
- **Check IPs from File:** Verify a list of IPs from a file and identify the corresponding providers.

### Supported Providers
* Amazon
* Cloudflare
* GitHub
* Google
* Microsoft
* OpenAI

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/BenjiTrapp/ip-to-cloudprovoder.git
   ```
2. Get the required dependencies
   ```bash
   go mod tidy
   ```
3. Build the binary
   ```
   make build
   ```

## Contribution
Contributions are welcome! If you'd like to add support for a new provider or improve the existing code, please submit a pull request.



**Not**e: This tool is provided as-is, without any warranties. Use it responsibly and respect the terms of service of the supported providers.


