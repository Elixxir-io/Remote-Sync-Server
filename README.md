# Remote Sync Server

The remote sync server is a self-run server for syncing Haven account data.

```yaml
# Path where log file will be saved.
logPath: "/tmp/remoteSyncServer.log"
# Level of debugging to print (0 = info, 1 = debug, >1 = trace).
logLevel: 1
# Port for Sync Server to listen on. It must be the only listener on this port.
port: 22841

# Path to CA-signed certificate files in PEM format.
signedCertPath: "~/syncServer.crt"
signedKeyPath:  "~/syncServer.key"

# Duration that logged-in sessions are valid.
tokenTTL: 24h
# Path to CSV containing list of authorized users in "<username>,<password>" format.
credentialsCsvPath: "~/credentials.csv"
# Base directory for synced files.
storageDir: "~/syncServer"
```
