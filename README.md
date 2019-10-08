# Kuberfoward

## Use

This script receives deployment information from configuration (deploy.yaml) file and as parameters.

It always connects to first pod of each deployment array.

Passing configuration as parameters:

```  
kubeforward <deploy_name>:<host_port>:<pod_port> [<deploy_name>:<host_port>:<pod_port> ...]
```

Passing configuration as config file:

```
deploy:
  - name: name
    hostport: 8080
    podport: 80

  - name: name
    hostport: <port>
    podport: <port>
```

The name of file must be deploy.yaml


## LICENSE

[GPLv3](https://www.gnu.org/licenses/gpl-3.0.html)
