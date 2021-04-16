This closes #

**Note to reviewers:** remember to look at the commits in this PR and consider if they can be squashed

## Summary Of Changes

## Additional Context

## Local Testing

Please ensure you run the unit, integration and system tests before approving the PR.

To run the unit and integration tests:

```
$ make unit-tests integration-tests
```

You will need to target a k8s cluster and have the operator deployed for running the system tests.

For example, for a Kubernetes context named `dev-bunny`:
```
$ kubectx dev-bunny
$ make destroy deploy-dev
# wait for operator to be deployed
$ make system-tests
``` 
