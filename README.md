[![IP Ranges Update](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/daily-scraper.yml/badge.svg)](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/daily-scraper.yml)
[![ipscanner](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/ipscanner.yml/badge.svg)](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/ipscanner.yml)
[![Quality Check after Commit](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/go.yml/badge.svg)](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/go.yml)

<p align="center">
<img height="200" src="static/logo.png">
<br> IP To CloudProvider
</p>

This command-line tool helps to manage and check IP ranges for various service providers. It allows you to update IP ranges for specific providers, check if an IP belongs to any provider's range, and even verify a list of IPs from a file. Some GitHub Actions are helping to create a nice workflow around the CLI-Tool.

## Features

- **Update IP Ranges:** Keep the IP ranges for various service providers up-to-date with a single command.
- **Check IP:** Determine if a specific IP belongs to the range of any supported provider.
- **Check IPs from File:** Verify a list of IPs from a file and identify the corresponding providers.

## GitHub Action Workflows
- **IP Scanner:** All IPs in this [file](https://github.com/BenjiTrapp/ip-to-cloudprovider/blob/main/ips_to_scan.txt) are validated and checked. After the check all info is send as a GitHub Issue against this repository. This helps for persisting the scan results and make it easier to use
- **Quality Checks:** After each merge into main or accepted PullRequest, quality checks are against the Code. In this way it makes things easier to identify a broken behavior
- **Daily Scraper:** Each day at midnight, this action get's triggered to update the IP ranges of the cloudproviders if something changed


## Demo
A picture says more then thousand words. Check out this demo (that you can redo on your own, check out the installation section below)

![](/static/demo.gif)

### Supported Providers
* Amazon
* Cloudflare
* GitHub
* Google
* GoogleCloud
* GoogleBot
* Microsoft
* OpenAI

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/BenjiTrapp/ip-to-cloudprovider.git
   ```
2. Get the required dependencies
   ```bash
   go mod tidy
   ```
3. Build the binary
   ```
   make build
   ```

## Usage Examples

#### Update all CloudProvider IP ranges
```bash
# use makefile
make update

# manual short
./ip-to-cloudprovider -a

# manual verbose
./ip-to-cloudprovider --update-all
```

#### Check a dedicated IP
```bash
./ip-to-cloudprovider check-ip <your IP>

   ____   ______     _______                _____               _    __       
  /  _/__/_  __/__  / ___/ /  ___  __ _____/ / _ \_______ _  __(_)__/ /__ ____
 _/ // _ \/ / / _ \/ /__/ /__/ _ \/ // / _  / ___/ __/ _ \ |/ / / _  / -_) __/
/___/ .__/_/  \___/\___/____/\___/\_,_/\_,_/_/  /_/  \___/___/_/\_,_/\__/_/   
   /_/                                                                        
-------------------------------------------------------
51.16.50.245    is in the range of Amazon
```

#### Check a List of ips 

Make sure that only one IP per line is present in your file. No seperator/delimiter is required, this makes it easier with some grep magic to create your base list that you want to check

```bash
./ip-to-cloudprovider check-file <your file with ips>

   ____   ______     _______                _____               _    __       
  /  _/__/_  __/__  / ___/ /  ___  __ _____/ / _ \_______ _  __(_)__/ /__ ____
 _/ // _ \/ / / _ \/ /__/ /__/ _ \/ // / _  / ___/ __/ _ \ |/ / / _  / -_) __/
/___/ .__/_/  \___/\___/____/\___/\_,_/\_,_/_/  /_/  \___/___/_/\_,_/\__/_/   
   /_/                                                                        
-------------------------------------------------------
51.16.50.245    is in the range of Amazon
130.211.30.225  is in the range of Google
20.15.240.179   is in the range of Openai
185.199.111.153 is in the range of Github
2400:cb00::     is in the range of Cloudflare
52.130.76.17    is in the range of Microsoft
```

## Contribution
Contributions are welcome! If you'd like to add support for a new provider or improve the existing code, please submit a pull request.



**Not**e: This tool is provided as-is, without any warranties. Use it responsibly and respect the terms of service of the supported providers.


