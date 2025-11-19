# Battery Dashboard Lambda Function

This AWS Lambda function provides a web dashboard for visualizing battery state predictions and charging schedules from the SmartCharge system.

## Features

- **Interactive Web Dashboard**: Displays a comprehensive chart showing:
  - Battery power predictions over time (line chart)
  - Predicted energy usage over time (line chart)  
  - Unit pricing as a dashed line
  - Day dividers marking midnight boundaries
  - Highlighted charging slots
  - Key statistics summary in left sidebar

- **Tabbed Navigation**: Multiple views accessible via top navigation:
  - Dashboard: Main line chart visualization
  - System States: Paginated, sortable data table of all system states

- **System States Table**: Full-featured data browser:
  - Paginated display (20 rows per page)
  - Sortable columns (ascending/descending)
  - ISO format date/time display
  - Real-time record count
  - Pagination controls

- **Pastel Color Scheme**: Soft, eye-friendly colors:
  - Pastel blue for battery power
  - Pastel yellow for usage predictions
  - Pastel red for pricing
  - Colors brighten on hover for interactive feedback

- **Interactive Chart Features**:
  - Proximity-based tooltips (appear when cursor near lines)
  - Line brightening on hover
  - Charging slot highlighting
  - Responsive sizing to fill available space
  - Day boundary lines for easy time reference

- **Desktop-First Layout**:
  - Sidebar with statistics and legend (fixed width)
  - Chart area expands to fill remaining space
  - Full-height viewport utilization
  - No fixed chart height limitations

- **JSON API**: Returns raw data in JSON format when requested

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

- `hours` (optional): Number of hours to display (default: 24, max: 730 days)
  - Example: `/dashboard?hours=48` for 48-hour view
  - Example: `/dashboard?hours=168` for 7-day view
  - Example: `/dashboard?hours=720` for 30-day view

- `fromDate` and `toDate` (optional): ISO date range filter (format: YYYY-MM-DD)
  - Takes precedence over `hours` parameter if both provided
  - End date is inclusive (extends to end of toDate at 23:59:59)
  - Example: `/dashboard?fromDate=2025-11-01&toDate=2025-11-15` for specific date range
  - Example: `/dashboard?format=json&fromDate=2025-10-01&toDate=2025-10-31` for monthly data export

- `format` (optional): Response format (`html` or `json`)
  - Example: `/dashboard?format=json`

- **JSON API-only parameters** (for `format=json`):
  - `page` (optional): Page number for pagination (default: 1, min: 1)
    - Example: `/dashboard?format=json&page=2`
  
  - `pageSize` (optional): Records per page (default: 100, max: 1000)
    - Example: `/dashboard?format=json&pageSize=50`
  
  - `sortBy` (optional): Column to sort by (default: `start_time`)
    - Valid values: `start_time`, `pv_estimate`, `predicted_usage`, `predicted_battery_power`, `unit_rate`, `will_charge`
    - Example: `/dashboard?format=json&sortBy=unit_rate`
  
  - `sortOrder` (optional): Sort direction (default: `asc`)
    - Valid values: `asc`, `desc`
    - Example: `/dashboard?format=json&sortOrder=desc`

### Combined Query Examples

```
# Get specific date range, page 2, 50 records per page, sorted by unit rate descending
/dashboard?format=json&fromDate=2025-11-01&toDate=2025-11-15&page=2&pageSize=50&sortBy=unit_rate&sortOrder=desc

# Get 7 days of data, page 2, 50 records per page, sorted by unit rate descending
/dashboard?format=json&hours=168&page=2&pageSize=50&sortBy=unit_rate&sortOrder=desc

# Get 30 days of data, sorted by battery power ascending
/dashboard?format=json&hours=720&sortBy=predicted_battery_power&sortOrder=asc

# Get charging periods only, sorted chronologically
/dashboard?format=json&sortBy=will_charge&sortOrder=desc
```

### GUI-based Date Range Filtering

In the **System States** tab:
- Use the **Date Range** picker at the top with two date inputs:
  - **First input**: Start date (required)
  - **Second input**: End date (optional - if left blank, uses the start date for a single day query)
- Click **Apply Filter** to fetch ALL data from the server for the selected date range
  - Automatically fetches all pages (up to 1000 records per request)
  - Combines all results into a single dataset for browsing
- Click **Reset** to reload the default 30-day range from today backwards
- The table automatically updates with the complete fetched data
- Pagination and sorting work within the filtered dataset
- **Use cases**:
  - Single day: Select date in first input, leave second blank → queries that specific date
  - Date range: Select both start and end dates → queries all data between the dates
  - Multi-month: Select dates spanning multiple months to analyze longer trends

## Visualization

The dashboard uses **D3.js** for lightweight, high-performance SVG-based rendering:
- Small bundle size (~75KB vs 3MB for traditional charting libraries)
- Fully responsive and mobile-friendly
- Custom-built interactive tooltips
- Smooth, native browser rendering

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
- **Pastel Blue line**: Battery power predictions (kW) showing charge/discharge cycles
- **Pastel Yellow line**: Predicted energy usage (kW) showing consumption patterns
- **Pastel Red dashed line**: Unit electricity pricing (p/kWh)
- **Light blue backgrounds**: Highlighted charging time slots when battery will charge
- **Day dividers**: Solid gray lines marking midnight boundaries between calendar days
- **Interactive tooltips**: Appear when cursor is within 30px of a line, showing ISO-formatted timestamp
- **Line brightening**: Closest line brightens and increases in width on hover
- **Charging highlights**: Charging slot backgrounds brighten when hovering over charging periods
- **Grid lines**: 30-minute interval markers (dashed) for precise time reference

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

## Recent Changes (November 19, 2025)

### Date Range Filtering (New Feature)
- **GUI-based date range selector** in System States tab:
  - Select arbitrary dates using browser date picker
  - "From Date" and "To Date" inputs with calendar UI
  - "Apply Filter" button to update table with selected range
  - "Reset" button to return to default 30-day view
  - Default range starts at 30 days ago through today
- **Backend support for date range queries**:
  - `fromDate` and `toDate` query parameters (format: YYYY-MM-DD)
  - Takes precedence over `hours` parameter when both provided
  - API queries can now fetch arbitrary historical date ranges
  - End date is inclusive (extends through end of toDate day)
- **Filtering enables**: Historical analysis, month-long reports, specific incident investigation
- **Works with pagination and sorting**: Filter results, sort by column, paginate through data

### Layout Redesign
- **Refactored to desktop-first layout** with sidebar navigation
- Sidebar (280px fixed width) contains statistics and legend
- Chart area expands to use all remaining viewport space
- Full-height utilization with no wasted space
- Navbar added at top with tab navigation

### Chart Improvements
- **Pastel color scheme** for softer, more eye-friendly visuals:
  - Battery: Pastel blue (#9BB9FF) → bright blue (#5B7FFF) on hover
  - Usage: Pastel yellow (#FFE9A8) → bright yellow (#FFD740) on hover
  - Price: Pastel red (#FFB3B3) → bright red (#FF8080) on hover
- **Price visualization changed** from bars to dashed line
- **Day dividers added** - solid lines at midnight marking day boundaries
- **Tooltip positioning fixed** - now accurately follows cursor relative to chart container
- **Line brightening** - closest line to cursor brightens and increases stroke width
- **Removed chart title** to maximize visualization space
- **Reduced top margin** from 80px to 20px for expanded chart area

### Navigation & Structure
- **Tab-based interface**: Dashboard (chart), System States (table), Analysis, Settings
- **Tab switching** with smooth transitions
- **Removed redundant title** from top-left (now in navbar tabs)
- **Navbar reorganization** - all navigation items aligned to left

### System States Table (New)
- **New "System States" tab** with full data browser
- **Paginated display**: 20 rows per page with navigation controls
- **Sortable columns**: Click any header to sort ascending/descending
- **ISO date format**: All timestamps displayed in ISO 8601 format
- **Record count**: Shows total records and current page range
- **Column headers**: 
  - Start Time (sortable)
  - PV Estimate (sortable)
  - Predicted Usage (sortable)
  - Battery Power (sortable)
  - Unit Rate (sortable)
  - Will Charge (sortable)

### Data Model Fixes
- **Battery capacity constraint**: Fixed issue where predicted_battery_power could exceed 9.5 kWh
  - Changed calculation order: charge is now added BEFORE the capacity cap
  - Battery power now correctly capped at 9.5 kWh maximum at all times
  - Prevents unrealistic values like 11.1 kWh

### Technical Improvements
- **Embedded Go templates**: All HTML/CSS/JS split into modular template files
- **Template organization**: Each component (dashboard, styles, scripts, chart, stats, etc.) in separate files
- **Responsive flexbox layout**: Uses CSS Grid and Flexbox for proper space distribution
- **Smart scrolling**: Sidebar scrollable independently, chart fills remaining space