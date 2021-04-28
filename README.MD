# KomodoHype Taxes

1. Install golang
```bash
wget https://golang.org/dl/go1.16.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.16.2.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

2. Clone the repo
```bash
cd /opt/
git clone https://github.com/troydota/bulldog-taxes.git
cd bulldog-taxes
```

3. Build the source
```bash
go build
```

4. Modify the config.
```bash
cp config.demo.yaml config.yaml && vim config.yaml
```

5. Setup auto run with systemboot.
```bash
sudo cp bulldog-taxes.service /etc/systemd/system && sudo systemctl start bulldog-taxes && sudo systemctl enable bulldog-taxes
```
