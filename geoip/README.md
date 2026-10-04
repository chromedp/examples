# geoip

The program asks a lookup service for the place of each IP address. Then it
shows the place and a map of the place in the terminal. It starts a local test
site that holds the service and the map, so it needs no internet. It needs a
terminal that can show images.

Run it from the root of the repository, with IP addresses as arguments. The
test site knows 200 private and documentation ranges:

```sh
$ go run ./geoip 10.3.4.5 203.0.113.70
```

Without arguments, the program looks up the address of this computer.

The flag `-url` sets the base URL of another site, for example a live site. The
program then does not start the test site. The selectors of the program are
written for the test site, so a live site can differ.
