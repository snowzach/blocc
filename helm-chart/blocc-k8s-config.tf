resource "google_container_cluster" "blocc" {
    name = "blocc"
    zone = "us-central1-c"
    description = "Elastic Kubernetes Cluster"

    min_master_version = "1.11.5-gke.5"
    node_version       = "1.11.5-gke.5"

    lifecycle {
      ignore_changes = ["node_pool"]
    }

    node_pool {
      name = "default-pool"
    }

    network    = "${google_compute_network.blocc-network.self_link}"
    subnetwork = "${google_compute_subnetwork.blocc-subnet.self_link}"
    ip_allocation_policy = {
        cluster_secondary_range_name  = "${google_compute_subnetwork.blocc-subnet.name}-pods"
        services_secondary_range_name = "${google_compute_subnetwork.blocc-subnet.name}-services"
    }
}

resource "google_container_node_pool" "np-elasticsearch-master" {
  name       = "np-elasticsearch-master"
  zone       = "us-central1-c"
  cluster    = "${google_container_cluster.blocc.name}"
  node_count = 0

  node_config {
    machine_type = "n1-standard-1"
    labels {
      app = "elasticsearch-master"
    }
    tags = ["elasticsearch"]
  }
}

resource "google_container_node_pool" "np-elasticsearch-ssd" {
  name       = "np-elasticsearch-ssd"
  zone       = "us-central1-c"
  cluster    = "${google_container_cluster.blocc.name}"
  node_count = 0

  node_config {
    machine_type = "n1-highmem-4"
    local_ssd_count = "2"
    labels {
      app = "elasticsearch-ssd"
    }
    tags = ["elasticsearch"]
  }
}

resource "google_container_node_pool" "np-elasticsearch-pd" {
  name       = "np-elasticsearch-pd"
  zone       = "us-central1-c"
  cluster    = "${google_container_cluster.blocc.name}"
  node_count = 0

  node_config {
    machine_type = "n1-highmem-4"
    labels {
      app = "elasticsearch-pd"
    }
    tags = ["elasticsearch"]
  }
}

resource "google_container_node_pool" "np-kibana" {
  name       = "np-kibana"
  zone       = "us-central1-c"
  cluster    = "${google_container_cluster.blocc.name}"
  node_count = 0

  node_config {
    machine_type = "n1-standard-1"
    labels {
      app = "kibana"
    }
    tags = ["kibana"]
  }
}

resource "google_compute_network" "blocc-network" {
    name                    = "blocc-network"
    auto_create_subnetworks = "true"
}

resource "google_compute_subnetwork" "blocc-subnet" {
    name                     = "blocc-network-subnet"
    region                   = "us-central1"
    network                  = "${google_compute_network.blocc-network.self_link}"
    ip_cidr_range            = "172.22.0.0/16"
    private_ip_google_access = true

    secondary_ip_range = {
        range_name    = "${google_compute_network.blocc-network.name}-subnet-pods"
        ip_cidr_range = "172.23.0.0/16"
    }

    secondary_ip_range = {
        range_name    = "${google_compute_network.blocc-network.name}-subnet-services"
        ip_cidr_range = "172.24.0.0/16"
    }
}

resource "google_compute_firewall" "blocc-subnet" {
  name        = "blocc-subnet"
  description = "Allow all hosts in the subnet to communicate with one another"
  network     = "${google_compute_network.blocc-network.name}"

  allow {
    protocol = "all"
  }

  source_ranges = ["172.22.0.0/16", "172.23.0.0/16", "172.24.0.0/16"]
}

resource "google_compute_firewall" "blocc-allowed" {
  name        = "blocc-allowed"
  description = "Access from outside"
  network     = "${google_compute_network.blocc-network.name}"

  allow {
    protocol = "tcp"
    ports    = ["22","32534","8080"]
  }

  source_ranges = ["76.188.97.67/32", "107.11.55.189/32"]
}

resource "kubernetes_service" "blocc_dns_lb" {
    metadata {
        name      = "blocc-dns-lb"
        namespace = "kube-system"

        labels {
            heritage = "Terraform"
            k8s-app  = "kube-dns"
        }

        annotations {
            "cloud.google.com/load-balancer-type" = "Internal"
        }
  }

  spec {
        selector {
            k8s-app = "kube-dns"
        }

        port {
            name        = "dns"
            port        = 53
            target_port = 53
            protocol    = "UDP"
        }

        session_affinity = "None"
        type             = "LoadBalancer"
    }
}

resource "kubernetes_service" "blocc_kibana_lb" {
    metadata {
      name      = "blocc-kibana-lb"
    }
    spec {
        selector {
            app = "kibana"
        }
        port {
            name        = "kibana"
            port        = 5601
            target_port = 5601
            protocol    = "TCP"
        }
        session_affinity = "ClientIP"
        type             = "LoadBalancer"
        load_balancer_source_ranges = ["76.188.97.67/32", "107.11.55.189/32"]
    }
}

resource "kubernetes_service" "blocc_esdata_lb" {
    metadata {
      name      = "blocc-esdata-lb"
    }
    spec {
        selector {
            esrole = "data"
        }
        port {
            name        = "elasticsearch"
            port        = 9200
            target_port = 9200
            protocol    = "TCP"
        }
        session_affinity = "None"
        type             = "LoadBalancer"
        load_balancer_source_ranges = ["76.188.97.67/32", "107.11.55.189/32", "172.22.0.0/16", "172.23.0.0/16", "172.24.0.0/16"]
    }
}

resource "kubernetes_service" "blocc_esdata_np" {
    metadata {
      name      = "blocc-esdata-np"
    }
    spec {
        selector {
            esrole = "data"
        }
        port {
            name        = "elasticsearch"
            port        = 9200
            target_port = 9200
            protocol    = "TCP"
        }
        type             = "NodePort"
    }
}

resource "null_resource" "get_cluster_credentials" {
  triggers = {
    google_cluster = "${google_container_cluster.blocc.id}"
  }

  provisioner "local-exec" {
    command = <<EOF
  sleep 120 \
  && gcloud config set project "coinninja-181718" \
  && gcloud config set compute/zone "us-central1-c" \
  && gcloud container clusters get-credentials blocc
EOF

    when = "create"
  }
}


output "gke_name" {
  value = "${google_container_cluster.blocc.name}"
}

output "gke_zone" {
  value = "${google_container_cluster.blocc.zone}"
}

output "gke_region" {
  value = "${google_container_cluster.blocc.zone}"
}

output "k8s_endpoint" {
  value = "${google_container_cluster.blocc.endpoint}"
}

output "k8s_client_certificate" {
  value = "${google_container_cluster.blocc.master_auth.0.client_certificate}"
}

output "k8s_client_key" {
  value = "${google_container_cluster.blocc.master_auth.0.client_key}"
}

output "k8s_cluster_ca_certificate" {
  value = "${google_container_cluster.blocc.master_auth.0.cluster_ca_certificate}"
}

output "blocc_dns_lb_ip" {
  value = "${kubernetes_service.blocc_dns_lb.load_balancer_ingress.0.ip}"
}

output "blocc_kibana_ip" {
  value = "${kubernetes_service.blocc_kibana_lb.load_balancer_ingress.0.ip}"
}
