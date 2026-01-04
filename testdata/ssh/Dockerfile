FROM alpine:latest

RUN apk add --no-cache openssh

# Generate host keys
RUN ssh-keygen -A

# Add test user
RUN adduser -D testuser && \
    echo "testuser:testpass" | chpasswd

# Prepare ssh config
RUN echo "PasswordAuthentication yes" >> /etc/ssh/sshd_config && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config

# Expose ssh port
EXPOSE 22

CMD ["/usr/sbin/sshd", "-D"]
