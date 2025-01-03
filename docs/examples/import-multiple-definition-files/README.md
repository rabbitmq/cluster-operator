# Import Multiple Definition Files Example

RabbitMQ supports importing definitions from a folder with multiple JSON files.
In this example we take advantage of this feature to split the definitions into a ConfigMap
with most of the definitions and a Secret that contains user definitions only.

First, we need to create the ConfigMap and Secret with the definitions:
```bash
kubectl create configmap definitions --from-file='definitions.json=/my/path/to/definitions.json'
kubectl create secret generic users --from-file='definitions.json=/my/path/to/users.json'
```

Afterwards, we leverage the StatefulSet Override to mount these resources as files in a folder
and specify that folder as the defintion import path.
