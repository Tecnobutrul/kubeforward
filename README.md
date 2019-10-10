# Kubefoward

## Use

This script receives deployment information from configuration (deploy.yaml) file and as parameters.

It always connects to first pod of each deployment array.

Configuration file path as well as aditional deployment info could be passed as parameters. The config file parameter should be the first.

Argument configuration will overwrite the file configuration in case both define same deployment name.

```  
kubeforward [-file=/conf/file/path] [<deploy_name>:<host_port>:<pod_port> ...]
```

Passing configuration as config file:

```
deploy:
  - name: name
    hostport: 8080
    podport: 80

  - name: <name>
    hostport: <port>
    podport: <port>
```



## LICENSE

[GPLv3](https://www.gnu.org/licenses/gpl-3.0.html)
