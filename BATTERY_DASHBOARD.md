# Battery Dashboard Lambda Function

A serverless web dashboard for visualizing SmartCharge battery state predictions and charging schedules.

## 🎯 Overview

This lambda function creates an interactive web dashboard that:

- **Visualizes battery predictions**: Shows predicted battery power over the next 24 hours as a line chart
- **Shows energy usage**: Displays predicted energy consumption as a separate line chart
- **Displays unit pricing**: Shows electricity pricing as background bars 
- **Highlights charging slots**: Identifies time periods when battery charging is scheduled
- **Provides statistics**: Shows key metrics like number of charge slots and price ranges
- **Auto-refreshes**: Dashboard automatically updates every 5 minutes
- **Mobile responsive**: Works on both desktop and mobile devices

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web Browser   │    │  Lambda Function │    │   DynamoDB      │
│                 │───▶│                 │───▶│ System States   │
│ Battery         │    │ - Query data    │    │ Table           │
│ Dashboard       │    │ - Generate HTML │    │                 │
│                 │◀───│ - Return chart  │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 📊 Dashboard Features

### Interactive Chart
- **Blue line**: Battery power predictions (kW) showing charge/discharge cycles
- **Teal line**: Predicted energy usage (kW) showing consumption patterns
- **Red bars**: Unit electricity pricing (p/kWh) as background context
- **Yellow bars**: Highlighted charging time slots when battery will charge

### Statistics Panel
- Total number of data points
- Number of planned charging slots  
- Maximum and minimum unit prices
- Last updated timestamp

### Query Parameters
- `hours=X`: Display X hours of data (default: 24, max: 168)
- `format=json`: Return raw JSON data instead of HTML dashboard

## 🌐 Access Methods

### 1. Direct Lambda Function URL
Fast, direct access with minimal latency:
```
https://[lambda-url].lambda-url.[region].on.aws/
```

### 2. API Gateway Endpoint  
Integrated with existing SmartCharge API:
```
https://[api-id].execute-api.[region].amazonaws.com/prod/dashboard
```

### Examples
```bash
# Default 24-hour dashboard
curl https://your-lambda-url/

# 48-hour view
curl https://your-lambda-url/?hours=48

# JSON data only
curl https://your-lambda-url/?format=json

# JSON with custom timeframe
curl https://your-lambda-url/?hours=12&format=json
```

## 🚀 Deployment

### Prerequisites
- AWS CLI configured with appropriate permissions
- Pulumi CLI installed
- Go 1.21+ installed

### Quick Deploy
```bash
# Deploy everything at once
./deploy-dashboard.sh
```

### Manual Deploy
```bash
# 1. Build lambda function
cd battery-dashboard
./build.sh

# 2. Deploy infrastructure  
cd ../infra
pulumi up
```

### Build Only
```bash
cd battery-dashboard
./build.sh
# Creates battery-dashboard.zip for manual upload
```

## 🔐 Required Permissions

The Lambda function needs these IAM permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:Query"
      ],
      "Resource": "arn:aws:dynamodb:REGION:ACCOUNT:table/SYSTEM_STATES_TABLE"
    },
    {
      "Effect": "Allow", 
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": "arn:aws:logs:REGION:ACCOUNT:*"
    }
  ]
}
```

## 🔧 Configuration

### Environment Variables
- `AWS_REGION`: AWS region for DynamoDB access
- `DDB_SYSTEM_STATES_TABLE_NAME`: DynamoDB table name for system states

### Lambda Settings
- **Runtime**: provided.al2 (custom Go runtime)
- **Handler**: bootstrap
- **Memory**: 128 MB (sufficient for dashboard generation)
- **Timeout**: 30 seconds
- **Architecture**: x86_64

## 📈 Usage Patterns

### Real-time Monitoring
- Access dashboard URL directly in browser
- Bookmark for quick access during peak/low tariff periods
- Monitor charging efficiency and schedule optimization

### Data Integration
- Use JSON API for custom applications
- Integrate with home automation systems
- Export data for analysis in other tools

### Mobile Access
- Responsive design works on phones/tablets
- Fast loading even on slower connections
- Touch-friendly interface for mobile interaction

## 🔍 Monitoring & Troubleshooting

### CloudWatch Logs
Lambda function logs are available in CloudWatch:
```
/aws/lambda/battery-dashboard-lambda
```

### Common Issues

1. **No data showing**: Check DynamoDB table has recent system states
2. **Timeout errors**: Verify DynamoDB table exists and Lambda has read permissions
3. **CORS issues**: Ensure API Gateway CORS settings are properly configured

### Health Check
```bash
# Test lambda function health
curl -f https://your-lambda-url/?hours=1

# Check JSON response format
curl https://your-lambda-url/?format=json | jq '.'
```

## 🎨 Customization

### Chart Styling
Modify the Chart.js configuration in `main.go` to:
- Change colors and styling
- Add additional data series
- Modify chart type (line, bar, area)
- Adjust time axis formatting

### Time Ranges
Default supports up to 7 days (168 hours). To extend:
1. Modify the `hours <= 168` validation in handler
2. Consider performance implications for larger datasets
3. May need to increase Lambda timeout for very large ranges

### Additional Metrics
Extend the dashboard by:
1. Adding new fields to the SystemState struct
2. Updating the HTML template
3. Modifying the Chart.js data preparation

## 📋 System States Data Structure

The dashboard expects this DynamoDB table structure:

```json
{
  "partition_key": 1,
  "start_time": 1697299200,
  "pv_estimate": 2.5,
  "predicted_usage": 1.8, 
  "predicted_battery_power": 0.7,
  "unit_rate": 15.2,
  "will_charge": false
}
```

Where:
- `start_time`: Unix timestamp of the time slot
- `pv_estimate`: Predicted solar PV generation (kW)
- `predicted_usage`: Predicted energy consumption (kW)
- `predicted_battery_power`: Net battery power (positive = charging, negative = discharging)
- `unit_rate`: Electricity unit price (pence per kWh)
- `will_charge`: Boolean indicating if battery will charge in this slot

## 🔄 Updates & Maintenance

### Updating the Lambda
1. Make code changes
2. Run `./build.sh`
3. Deploy with `pulumi up`

### Monitoring Performance
- Check CloudWatch metrics for invocation count and duration
- Monitor DynamoDB read capacity if using provisioned billing
- Set up CloudWatch alarms for error rates

### Data Retention
The dashboard queries up to 7 days of future predictions. Ensure your data pipeline populates the DynamoDB table with sufficient future data points for optimal user experience.