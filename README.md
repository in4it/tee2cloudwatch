# tee2cloudwatch
Send your output data of your commandline to cloudwatch logs. Works well in combination with AWS UserData scripts. You can send the output data of your initialization scripts straight to cloudwatch logs instead of writing them in /var/log.

# Usage
```
Usage of ./tee2cloudwatch:
  -logGroup string
        log group name
  -region string
        region
```
# Example

```
#!/bin/bash -e
exec > >(./tee2cloudwatch -logGroup test -region eu-west-1) 2>&1
  echo "Hello from user-data!"
  echo "this is another echo"
  echo "..."
  echo "more messages"
  sleep 2
  echo "..."
  echo "more messages"
  sleep 30
  echo "and one more... goodbye!"
```