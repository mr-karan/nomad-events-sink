job "nomad-events-sink" {
  datacenters = ["dc1"]
  type        = "service"

  group "app" {
    count = 1

    ephemeral_disk {
      migrate = true
      size    = 500
      sticky  = true
    }

    restart {
      attempts = 2
      interval = "2m"
      delay    = "30s"
      mode     = "fail"
    }

    task "app" {
      driver = "docker"

      config {
        image = "ghcr.io/mr-karan/nomad-events-sink:latest"
      }

      env {
        NOMAD_EVENTS_SINK_app__data_dir = "/alloc/data/"
        // Ensure this interface is where Nomad advertises it's HTTP port.
        NOMAD_ADDR = "http://${attr.nomad.advertise.address}"
      }

      resources {
        cpu    = 200
        memory = 200
      }
    }
  }
}
