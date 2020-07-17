# Import Definitions Example

You can import definitions, which contains definitions of all broker objects from a local file at Node Boot. (Learn more about export and import rabbitmq definitions)[https://www.rabbitmq.com/definitions.html#import].

You can save the export definitions json file in a ConfigMap object in like so

```yaml
kind: ConfigMap
metadata:
  name: definitions
  namespace: # should be the same namespace as the rabbitmqcluster instance you are importing the definitions to
data:
  def.json: |
# exported definitions context
```

and then leverage the StatefulSet Override to mount this additional ConfigMap `definitions` to your rabbitmqcluster instance. Check out `rabbitmq.yaml` as an example.

Keep in mind that exported definitions contains all broker objects, including users. This means that the admin user credentials will be imported from the definitions, and will not be the one which is generated at the creation of the deployment as a kubernetes secret object.
