# SmokePing Example

This is an example of running SmokePing alongside Surveyor. It assumes the following directory structure:

```bash
$ tree .
.
├── docker-compose.yml
├── nginx.conf
├── smokeping
│   └── ...
└── surveyor
    └── ...
```
    
Start with:

```bash
docker-compose up --build --detach
```

SmokePing is mounted at `/` and is available at `localhost/smokeping/` by default and Surveyor at `localhost/surveyor/`