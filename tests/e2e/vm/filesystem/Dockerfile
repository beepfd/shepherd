FROM ubuntu:22.04

RUN apt-get update \
    # install systemd as initialization module
    && apt-get install --no-install-recommends --no-install-suggests -y systemd \
    # install ssh to allow connect from outside
    && apt-get install -y openssh-server \
    # install net-tools to enable eth0 network interface
    && apt-get install --no-install-recommends --no-install-suggests -y net-tools \
    && rm -rf /var/lib/apt/lists/*

# switch initialization target from GUI (graphical.target) to text (multi-user.target) mode
RUN cd /lib/systemd/system && ln -sf multi-user.target default.target

# enable serial port to use for login
RUN systemctl enable getty@ttyS0.service
# enable ssh server
RUN systemctl enable ssh.service

# set root password
RUN echo "root:root" | chpasswd

# enable autologin on serial port
RUN sed -i 's/ExecStart=.*/ExecStart=-\/sbin\/agetty --noissue --autologin root %I $TERM/g' /lib/systemd/system/getty@.service
# keep boot messages on tty console
RUN sed -i 's/TTYVTDisallocate=yes/TTYVTDisallocate=no/g' /lib/systemd/system/getty@.service
# allow login as root through ssh
RUN sed -i 's/.*PermitRootLogin.*/PermitRootLogin yes/g' /etc/ssh/sshd_config

# set hostname
RUN echo "shepherd" > /etc/hostname

COPY key.pem /root/
COPY cert.pem /root/
COPY https_server.py /root/
COPY https_client.py /root/

# enable eth0
RUN cat <<EOF >> /etc/systemd/system/eth0.service
    [Unit]
    Description=eth0 service

    [Service]
    User=root
    WorkingDirectory=/root
    Type=oneshot
    ExecStart=ifconfig eth0 10.0.2.15 netmask 255.255.255.0
    ExecStart=route add default gw 10.0.2.2
    ExecStart=/bin/bash -c '/usr/bin/echo nameserver 8.8.8.8 > /etc/resolv.conf'
    ExecStart=/bin/bash -c '/usr/bin/echo "10.0.2.15 shepherd.domain" >> /etc/hosts'
    ExecStart=/bin/bash -c '/usr/bin/echo "127.0.0.1 shepherd-tls12.domain" >> /etc/hosts'
    ExecStart=/bin/bash -c '/usr/bin/echo "10.0.2.15 shepherd-tls13.domain" >> /etc/hosts'

    [Install]
    WantedBy=multi-user.target
EOF
RUN systemctl enable eth0.service

#start python https server
RUN cat <<EOF >> /etc/systemd/system/https_server.service
    [Unit]
    Description=https server service

    [Service]
    User=root
    WorkingDirectory=/root
    Type=oneshot
    ExecStart=/bin/bash -c 'python3.10 /root/https_server.py'

    [Install]
    WantedBy=multi-user.target
EOF
RUN systemctl enable https_server.service

# prepare python https client
RUN cat <<EOF >> /etc/systemd/system/https_client.conf
    HOST_TLS12=shepherd-tls12.domain
    HOST_TLS13=shepherd-tls13.domain
    PORT=443
EOF

#start python https client
RUN cat <<EOF >> /etc/systemd/system/https_client.service
    [Unit]
    Description=https client service

    [Service]
    EnvironmentFile=/etc/systemd/system/https_client.conf
    User=root
    WorkingDirectory=/root
    Type=oneshot
    ExecStart=/bin/bash -c 'python3.10 /root/https_client.py'

    [Install]
    WantedBy=multi-user.target
EOF
RUN systemctl enable https_client.service

RUN cat <<EOF >> /etc/systemd/system/shepherd.service
    [Unit]
    Description=shepherd service

    [Service]
    User=root
    WorkingDirectory=/root
    Type=idle
    ExecStart=/root/shepherd --config-path /root/config.yaml

    [Install]
    WantedBy=multi-user.target
EOF