# Battery Dashboard Lambda Function

This AWS Lambda function provides a web dashboard for visualizing battery state predictions and charging schedules from the SmartCharge system.

## Features

- **Interactive Web Dashboard**: Displays a comprehensive chart showing:
  - Battery power predictions over time (line chart)
  - Predicted energy usage over time (line chart)  
  - Unit pricing as background bars
  - Highlighted charging slots
  - Key statistics summary

- **JSON API**: Returns raw data in JSON format when requested

- **Responsive Design**: Works on desktop and mobile devices

- **Auto-refresh**: Dashboard automatically refreshes every 5 minutes

## API Usage

### Web Dashboard (Default)
```
GET /dashboard
```
Returns an HTML page with the interactive dashboard.

### JSON Data
```
GET /dashboard?format=json
```
or
```
GET /dashboard
Accept: application/json
```
Returns raw system state data in JSON format.

### Query Parameters

- `hours` (optional): Number of hours to display (default: 24, max: 168)
  - Example: `/dashboard?hours=48` for 48-hour view

- `format` (optional): Response format (`html` or `json`)
  - Example: `/dashboard?format=json`

## Response Format

### JSON Response
```json
{
  "system_states": [
    {
      "start_time": "2025-10-14T10:00:00Z",
      "pv_estimate": 2.5,
      "predicted_usage": 1.8,
      "predicted_battery_power": 0.7,
      "unit_rate": 15.2,
      "will_charge": false
    }
  ],
  "generated": "2025-10-14T10:15:30Z"
}
```

### Interactive Chart
- **Blue line**: Battery power predictions (kW) showing charge/discharge cycles
- **Teal line**: Predicted energy usage (kW) showing consumption patterns
- **Red bars**: Unit electricity pricing (p/kWh) as background context
- **Yellow bars**: Highlighted charging time slots when battery will charge

## Environment Variables

The lambda function requires these environment variables:

- `AWS_REGION`: AWS region for DynamoDB access
- `DDB_SYSTEM_STATES_TABLE_NAME`: Name of the DynamoDB table containing system states

## Building

Run the build script to create a deployment package:

```bash
./build.sh
```

This creates `battery-dashboard.zip` ready for AWS Lambda deployment.

## Deployment

1. Upload `battery-dashboard.zip` to AWS Lambda
2. Set runtime to "Provided AL2"
3. Configure environment variables
4. Set up API Gateway trigger (optional)
5. Configure IAM role with DynamoDB read permissions

## Required Permissions

The Lambda execution role needs:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:Query"
      ],
      "Resource": "arn:aws:dynamodb:REGION:ACCOUNT:table/TABLE_NAME"
    }
  ]
}
```

## Development

To run locally for testing:

```bash
go run main.go
```

Note: You'll need AWS credentials configured and the DynamoDB table accessible.