# Battery Dashboard Update: Predicted Usage Line

## Summary
Updated the battery dashboard to include predicted energy usage as a new line chart, providing a more comprehensive view of energy patterns alongside battery state predictions.

## Changes Made

### 1. Chart Data Enhancement
- **Added Predicted Usage Data**: New `predictedUsageData` array extracting `predicted_usage` from system states
- **New Dataset**: Added teal-colored line chart for predicted energy usage
- **Shared Axis**: Both battery power and predicted usage use the same Y-axis (kW) for easy comparison

### 2. Visual Updates
- **New Legend Item**: Added teal legend item for "Predicted Usage (kW)"
- **Chart Title**: Updated from "Battery State Prediction & Charging Schedule" to "Battery State, Usage Predictions & Charging Schedule"
- **Axis Label**: Changed left Y-axis from "Battery Power (kW)" to "Power (kW)" to reflect dual usage
- **Color Scheme**: Used distinctive teal color (#4bc0c0) for usage line to avoid confusion

### 3. Technical Details
- **Chart.js Configuration**: Added new line dataset with proper styling
- **Data Processing**: Maps `predicted_usage` field from system states, defaulting to 0 if null
- **Consistent Styling**: Matches battery power line styling (line chart, no fill, tension 0.1)

### 4. Architecture Update
- **Build Target**: Temporarily switched battery dashboard from ARM64 to AMD64 to resolve compilation issue
- **Lambda Configuration**: Updated infrastructure to use x86_64 architecture for battery dashboard

### 5. Documentation Updates
- **README.md**: Updated feature descriptions to mention predicted usage line
- **BATTERY_DASHBOARD.md**: Enhanced documentation with new chart element descriptions
- **Visual Guide**: Added teal line description to interactive chart section

## Chart Elements (Updated)

The dashboard now displays:

1. **Blue Line (Battery Power)**: Predicted battery charge/discharge in kW
2. **Teal Line (Predicted Usage)**: Expected energy consumption in kW  
3. **Red Bars (Unit Price)**: Electricity pricing in p/kWh as background
4. **Yellow Bars (Charging Slots)**: Highlighted periods when charging is scheduled

## Benefits

### Enhanced Insights
- **Energy Balance**: Clear view of consumption vs. battery contribution
- **Pattern Recognition**: Easier to spot usage peaks and valleys
- **Optimization Validation**: Visual confirmation that charging aligns with usage patterns
- **Grid Import Calculation**: Difference between usage and battery power shows grid dependency

### User Experience
- **Comprehensive View**: Single chart shows all key energy metrics
- **Color Coding**: Distinctive colors prevent confusion between metrics
- **Shared Scale**: Direct comparison of battery power and usage magnitudes
- **Mobile Friendly**: Responsive design maintains readability on all devices

## Usage Scenarios

### Daily Planning
- **Morning Peak**: See if battery can cover morning usage surge
- **Evening Demand**: Check battery availability during peak evening consumption
- **Low Usage Periods**: Identify optimal charging windows when usage is minimal

### Optimization Analysis
- **Charge Timing**: Verify charging occurs when usage is low or prices are cheap
- **Discharge Efficiency**: Confirm battery discharge aligns with high usage periods
- **Grid Independence**: Monitor periods when battery + usage minimize grid dependency

## Deployment

The updated dashboard is now live at:
- **Direct URL**: https://kr4fe67aktrjmfyocvfhqr74sy0wdgks.lambda-url.eu-west-1.on.aws/
- **API Gateway**: https://u3u3ub14kc.execute-api.eu-west-1.amazonaws.com/prod/dashboard

## Technical Notes

### Data Requirements
The dashboard expects `predicted_usage` field in DynamoDB system states table:
```json
{
  "partition_key": 1,
  "start_time": 1697299200,
  "predicted_usage": 1.8,  // kW consumption prediction
  "predicted_battery_power": 0.7,  // kW battery charge/discharge
  "unit_rate": 15.2,  // p/kWh pricing
  "will_charge": false  // charging schedule
}
```

### Performance
- **Minimal Impact**: Adding one data series has negligible performance effect
- **Same Query**: No additional DynamoDB queries required
- **Efficient Rendering**: Chart.js handles multiple datasets efficiently

### Future Enhancements
- **PV Generation**: Could add solar PV prediction line
- **Net Import/Export**: Calculate and display grid import/export
- **Stacked Areas**: Option to show cumulative energy flows
- **Energy Balance**: Real-time energy balance calculations