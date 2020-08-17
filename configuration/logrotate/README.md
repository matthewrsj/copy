# Logrotate Configuration

Make sure `cron` and `logrotate` are installed then install the following files
- towercontroller
  - /etc/logrotate.d/towercontroller
- docker-container
  - /etc/logrotate.d/docker-container
- crontab-e
  - run `crontab -e` and add the configuration to the end of the file
  - will need to change path name if user is not `mfgtest`
