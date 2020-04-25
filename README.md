# What...day is it again?

This is a little service I created that will text you the day of the week every morning. Why is this relevant, you might ask? Well, in the beginning of 2020 (I'm assuming you're reading this far in the future...) we all had to stay home because of a nation-wide pandemic. Days turned in to weeks, weeks turned in to months, and generally everything blurred together. I, personally, began to even forget what day of the week it was!

So this service was born! It's a bit over-engineered because I also wanted to take this opportunity to describe how I'd build a modern web app on Golang using AWS. It uses ECS for compute, DynamoDB primarily for transactional storage, and Route53 for DNS. You can check out the accompanying [CloudFormation template](./cloudformation/template.yaml) for a better description of how everything plays nicely together.

# Development

There are really two projects in one here.

## Dependencies

* golang 1.13 (at least)
* node v10.x.x
* npm v6.x.x


## Golang

You might need to install go-bindata.

```
brew install go-bindata
```

You can build the Golang target by using the `build` make target.

```bash
$ make build
```

This will output a binary you can use in development. Use the `-development` flag to put the app in development mode (which will load some assets off disk instead of out of memory...more on that later).

## JavaScript

There's a JavaScript app that powers the front-end and is embedded in the resultant binary as part of the build process. You can find it under `pkg/ui` and it behaves like a normal JavaScript app--build with `npm run build` and run the app in development mode using `npm start`.

# Deployment

Infrastructure is managed soley by CloudFormation. The following snippets are provided for convenience sake.

## Creating a stack

**NOTE:** Fill in the parameter values below.

```bash
$ aws cloudformation create-stack --stack-name=what-day-is-it-1 \
    --parameters ParameterKey=TwilioAccountSID,ParameterValue=... ParameterKey=TwilioAuthToken,ParameterValue=... ParameterKey=TwilioPhoneNumber,ParameterValue=...  ParameterKey=Environment,ParameterValue=production \
    --template-body=file://./cloudformation/template.yaml \
    --capabilities=CAPABILITY_NAMED_IAM
```

## Updating a stack

```bash
$ aws cloudformation update-stack --stack-name=what-day-is-it-2 \
    --parameters ParameterKey=TwilioAccountSID,UsePreviousValue=true ParameterKey=TwilioAuthToken,UsePreviousValue=true ParameterKey=TwilioPhoneNumber,UsePreviousValue=true  ParameterKey=Environment,UsePreviousValue=true \
    --template-body=file://./cloudformation/template.yaml \
    --capabilities=CAPABILITY_NAMED_IAM
```

# Contributing

If, for some weird reason, you would like to contribute just open a pull request! I'm happy to accept PRs.
