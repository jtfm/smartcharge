
# Data gathering Phase

## Pricing Data Gathering Phase
1. Get pricing info from the DB. **DONE**
2. If not available from db, get next 24 hours worth of pricing from octopus API **DONE**
  - If not available from Octopus, throw an exception and log to cloudwatch **DONE** (except CW)

## Usage Data Gathering Phase
5. Get usage data from the Octopus API **DONE**
6. Store the usage in the DB **DONE**

## Solar Forecast Data Gathering Phase
4. Look up solar forecasts from DB **DONE**
5. If solar forecast from DB is not available, call the solcast API and store the forecast in the DB **DONE**

# Planning Phase
6. Compare predicted generation over the next 24 hours with the estimated usage, taking into account predicted charging. Generate the predicted battery levels over the next 24 hours. **DONE**
7. If the predicted battery levels over the next twenty four hours will go below zero without grid charging (excluding less-than-zero charging), plan the cheapest times to charge to avoid this happening. **DONE**
8. Log the plan to the database. **DONE**

# Action Phase
8. If the plan includes charging now, call the inverter API and command it to start charging. **DONE** 

# Housekeeping Phase
9. Delete any pricing data, forecast data and planning data more than a month old.
