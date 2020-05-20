pipeline {
    agent {
        dockerfile {
            filename "Dockerfile"
            args "--network=host"
        }
    }

    triggers { pollSCM('') }
    options {
        timeout(time: 30, unit: 'MINUTES')
    }
    stages {
        stage ('Lint') {
            environment {
                XDG_CACHE_HOME = "/tmp/.cache"
                CGO_ENABLED = "0"
                GOPATH = "$HOME/go"
            }
            steps {
                deleteDir()
                checkout scm
                sh 'golangci-lint run ./...'
            }
        }
        stage ('Test') {
            environment {
                XDG_CACHE_HOME = "/tmp/.cache"
                CGO_ENABLED = "0"
                GOPATH = "$HOME/go"
            }
            steps {
                deleteDir()
                checkout scm
                sh 'go test -gcflags=-l -mod vendor -cover ./...'
            }
        }
    }
}
