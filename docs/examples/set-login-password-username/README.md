# Set login password username Example

This is a sample will guide you how to override rabbitMq clustered default password & username.

Modify `default_user` be your login username, `default_pass` be your login password.

```yaml
spec:
  rabbitmq:
    additionalConfig: |
      default_user=guest
      default_pass=guest
```

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```
