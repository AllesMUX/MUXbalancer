# MUXbalancer
MUXbalancer is a reverse proxy application for load balancing between your work servers. It supports two balancing methods: Round Robin in case the path and method of request to the server are not specified in the configuration file, and Least Load if the corresponding entry is specified in the configuration. To get the load status, you need to integrate the [MUXworker](https://github.com/AllesMUX/MUXworker) module into your project.
To work, you need to additionally install Redis and specify its settings in the `config.yaml` configuration file.

## Installation
To install MUXbalancer, use the following command:

`go get github.com/AllesMUX/MUXbalancer`

## Usage
To use MUXworker in your worker service, first setup all your worker endpoints and request methods in `config.yaml`, for example:
```yaml
  balance:
    - path: /process
      method: POST
```
This indicates that `POST` request to `/process` should find the least loaded server.

Integrate [MUXworker](https://github.com/AllesMUX/MUXworker) in your worker application.

Setup Redis and you are ready to go!

## Contributing
Contributions to MUXbalancer are welcome! If you have an idea for a new feature or have found a bug, please open an issue on the [GitHub issue tracker](https://github.com/AllesMUX/MUXbalancer/issues).

If you would like to contribute code, please fork the repository and submit a pull request.

## License
MUXbalancer is licensed under the [Apache License 2.0](https://github.com/AllesMUX/MUXbalancer/blob/main/LICENSE).