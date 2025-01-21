# HearthHub Mod API
An API running on the dedicated linux machine for interfacing with the Valheim server. This repository
contains a docker image which runs the Valheim server. Unlike the docker image that comes pre-packaged with the Valheim
dedicated server this image installs the server directly onto the image rather than running a separate 
script in a generic ubuntu image.

This means that the dedicated server arguments (i.e world name, password, crossplay etc...) can be modified when the image is deployed or run.

## Building

To build the docker image run: `docker build . -t hearthhub-server:0.0.1` replacing `0.0.1` with
the image version you would like to use. 

## Running

You can run the Valheim dedicated server with `./start_server_docker.sh`

## Deployment

Deployment is managed through Helm.

## Built With

- [Kubernetes](https://kubernetes.io) - Container orchestration platform
- [Helm](https://helm.sh) - Manages Kubernetes deployments
- [Docker](https://docker.io/) - Container build tool
- [Stean](https://steam.com) - CLI used to install Valheim dedicated server

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code
of conduct, and the process for submitting pull requests to us.

## Versioning

We use [Semantic Versioning](http://semver.org/) for versioning. For the versions
available, see the [tags on this
repository](https://github.com/cbartran/hearthhub-mod-api/tags).

## Authors

- **cbartram** - *Initial work* -
  [cbartram](https://github.com/cbartram)

## License

This project is licensed under the [CC0 1.0 Universal](LICENSE)
Creative Commons License - see the [LICENSE.md](LICENSE) file for
details
