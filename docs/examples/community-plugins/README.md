# Community Plugins Example

**WARNING**: this example proves it is possible to enable a community plugin that is not included in the `rabbitmq` image. However, it relies on being able to download a file on cluster startup. If that URL is unavailable, your cluster won't start. Moreover, the downloaded file is not validated in any way and therefore you could end up loading arbitrary, potentially malicious, file into your cluster.

We'll keep investigating alternative options but for the time being, creating a custom image with the required plugins and just enabling them through `additionalPlugins` seems like a better idea.

**NOTE**: Please raise issues related to community plugins with the community - our team does not maintain these plugins.

You can deploy this example like this:

```shell
kubectl apply -f rabbitmq.yaml
```
