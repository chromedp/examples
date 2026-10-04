# geoip

The program looks up the place of each IP address offline, in the embedded
GeoLite2 City database of MaxMind. Then it shows the place and a map of the
place in the terminal. It takes the map from a local test site that it starts,
so it needs no internet. It needs a terminal that can show images.

Run it from the root of the repository, with IP addresses as arguments:

```sh
$ go run ./geoip 81.2.69.142 2001:4860:4860::8888
```

Without arguments, the program looks up the example address 8.8.8.8.

The flag `-l` sets the language of the place names. The database has names in
several languages, for example `de`, `ja` and `ru`. The flags `-zoom` and
`-scale` change the map.

The flag `-url` sets the base URL of another map site. The program then does not
start the test site. The tiles and the selectors of the program are written for
the test site, so a live site can differ.
