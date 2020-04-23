# Deployment

## Creating a stack

```bash
$ aws cloudformation create-stack --stack-name=what-day-is-it-1 \
    --parameters=ParameterKey=TwilioAccountSID,ParameterValue=... ParameterKey=TwilioAuthToken,ParameterValue=... ParameterKey=TwilioPhoneNumber,ParameterValue=... ParameterKey=Environment,ParameterValue=production \
    --template-body=file://./cloudformation/template.yaml \
    --capabilities=CAPABILITY_NAMED_IAM
```

## Updating a stack

```bash
$ aws cloudformation create-stack --stack-name=what-day-is-it-1 \
    --parameters=ParameterKey=TwilioAccountSID,UsePreviousValue=true ParameterKey=TwilioAuthToken,UsePreviousValue=true ParameterKey=TwilioPhoneNumber,UsePreviousValue=true ParameterKey=Environment,UsePreviousValue=true \
    --template-body=file://./cloudformation/template.yaml \
    --capabilities=CAPABILITY_NAMED_IAM
```