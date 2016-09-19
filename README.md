# ji-marina

The `marina` is a simple web server to publish the `piscineri3/*` `Docker` images from the `Docker` hub.

## Example

To retrieve an image from this proxy, one can use the `docker load` command:

```sh
$> curl -X GET  http://piscine.in2p3:8080/docker-images/piscineri3/go-base:latest | docker load
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  490M    0  490M    0     0  25.8M      0 --:--:--  0:00:18 --:--:-- 12.4M
Loaded image: piscineri3/go-base:latest
```

### Using `marina-pull`

A simple `Go` command is also available:

```sh
$> go get github.com/clr-info/ji-marina/cmd/marina-pull
$> marina-pull piscineri3/go-base:latest
Loaded image: piscineri3/go-base:latest
```
