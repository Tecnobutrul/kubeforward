# Kubeforward

kubeforward is a command line utility built to port forward some or all pods within a Kubernetes namespace. kubeforward uses the same port exposed by the service and forwards it from a loopback IP address on your local workstation. It loads all the configurations from a yml file so the configuration is quite easy.

When developing on our local workstation, you often build applications that need to access services through ports within a Kubernetes namespace. kubefwd allows us to develop locally with services available as we would be in the cluster.

## Installation

In order to use `kubeforward` you can easily install it by issuing the command below: 

```
go get -u -v github.com/Tecnobutrul/kubeforward
```

## Use

This script receives deployment information from configuration (deploy.yaml) file and as parameters.

It always connects to first pod of each deployment array.

Passing configuration file and literals as parameters. In case you pass the file config as argument, it **should** be the first one:

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
