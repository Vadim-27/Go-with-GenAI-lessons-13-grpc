# VM для домашнього завдання DevOps: Ubuntu 22.04 + Docker + k3s + AWS CLI (для Floci) + Go.
# Запуск: vagrant up   |   shell: vagrant ssh   |   зупинити: vagrant halt   |   видалити: vagrant destroy -f
#
# Версія box зафіксована для відтворюваності (слайд 6); перевірити: `vagrant box list`.
BOX_VERSION = ENV.fetch("BOX_VERSION", "20241002.0.0")

Vagrant.configure("2") do |c|
  c.vm.box = "ubuntu/jammy64"
  c.vm.box_version = BOX_VERSION unless BOX_VERSION.empty?
  c.vm.hostname = "devops-hw"

  # Порти на хост (Windows): http://localhost:<host>. 8080 на хості зайнятий іншими проєктами в Docker Desktop.
  c.vm.network "forwarded_port", guest: 8080,  host: 18080, host_ip: "127.0.0.1" # LibraTrack у Docker Compose
  c.vm.network "forwarded_port", guest: 30080, host: 30080, host_ip: "127.0.0.1" # LibraTrack у k3s (NodePort)
  c.vm.network "forwarded_port", guest: 80,    host: 8088,  host_ip: "127.0.0.1" # Traefik Ingress у k3s

  c.vm.synced_folder ".", "/app"

  # Під Hyper-V (VirtualBox у режимі NEM) завантаження повільніше, ніж 5 хв за замовчуванням.
  c.vm.boot_timeout = 900

  c.vm.provider "virtualbox" do |vb|
    vb.name   = "devops-hw"
    vb.memory = 3072
    # 1 CPU: під Hyper-V Ubuntu з 2 vCPU зависає на старті (одразу після підключення дисків).
    vb.cpus   = 1
    # З Hyper-V паравіртуальний годинник KVM не працює ("TSC mode not suitable"):
    # час у гостьовій ОС зупиняється, і Ubuntu зависає при старті. Вимикаємо його.
    vb.customize ["modifyvm", :id, "--paravirt-provider", "none"]
  end

  # Ідемпотентний скрипт: повторний `vagrant provision` лише перевіряє і доставляє відсутнє.
  c.vm.provision "shell", path: "scripts/provision.sh"
end
