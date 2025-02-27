# Smartcharge Application
This application is a simple project that joins data from the Octopus API, the Solcast API and the 
Givenergy API to predict the best times to charge my Givenergy home battery. You may find it useful
if you have a similar setup, or want to learn how to use the APIs or build a similar application.

## Summary
- It runs as a regularly scheduled lambda functions on AWS every 30 minutes. 
- It logs to CloudWatch, and the cloudwatch logs are deleted after 30 days.
- All resources are tagged by User, Stack Name and StackId to enable easy cost tracking and management.
- All resources are created using Pulumi IoC in nodejs within the /infra directory as code to enable easy deployment and management.

## What it does
1. It queries the Octopus API for pricing data, the Solcast API for solar forecast data and the Givenergy API for usage data. 
2. It stores this data in a DynamoDB table to avoid hitting the APIs too frequently.
3. Once it has the data, it compares the predicted solar generation with the predicted usage to estimate the battery levels over the next 24 hours. 
4. If it predicts that battery level will go below zero at any point in the next 24 hours, it plans the cheapest times to charge the battery to avoid this happening. It also writes this plan to the database.
5. It calls the Givenergy API to start or stop charging the battery based on the plan.

## Deployment
To deploy the application, you will need:
- A Givenergy solar and battery system
- A Givenergy API key
- An Octopus Energy account and API key
- A Solcast account (as a home user)
- An AWS account with admin access
- NodeJS installed on your machine
- Pulumi installed on your machine

1. Clone the repository to your local machine.
2. Run ```npm install``` in the /infra directory to install the required dependencies.
3. Configure the required variables for pulumi config with ```pulumi config set [configName] [configValue]```. All required variables are listed in the /infra/pulumi.yaml file.
4. Run "pulumi up" from the /infra directory.
