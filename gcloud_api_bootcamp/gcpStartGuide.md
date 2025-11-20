## This document covers the GCP api setup steps
---

### Install gcloud CLI
---

#### References
---
- https://docs.cloud.google.com/sdk/docs/install

#### Commands
---
```
- sudo apt-get update
- sudo apt-get install apt-transport-https ca-certificates gnupg curl
- curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg
- echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | sudo tee -a /etc/apt/sources.list.d/google-cloud-sdk.list
- sudo apt-get update && sudo apt-get install google-cloud-cli
- sudo apt-get install google-cloud-cli-app-engine-go
```

### Launch the gcloud project
---
```
- gcloud init
- gcloud auth login --no-launch-browser     >>>>  this will give a link, paste in browser and paste the password
- gcloud config set project gpchandson-api  >>>>  Replace gchandson-api with ur project label
```

### Running the program
---

#### Refrences
---
- https://docs.cloud.google.com/storage/docs/reference/libraries#client-libraries-install-go

#### Details of the program
---
- program is stored as storage_quickstart.go
- go run storage_quickstart.go

```
Note: There might be additional authentication required
gcloud auth application-default login --no-launch-browser

Description in the link: https://docs.cloud.google.com/docs/authentication/set-up-adc-local-dev-environment
```
