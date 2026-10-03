# geoip

The program looks up each IP address in a database that it embeds. Then it
shows the place and a map of the place in the terminal. It reads Google Maps,
so it needs the internet and a terminal that can show images.

Run it from the root of the repository, with IP addresses as arguments:

```sh
$ go run ./geoip $(curl -s ifconfig.me) 92.87.44.226 75.181.186.168
```
