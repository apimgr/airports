// Jenkinsfile for Airports API Server
// Multi-architecture build pipeline (amd64, arm64)
// Server: jenkins.casjay.cc

pipeline {
    agent none

    environment {
        PROJECTNAME = 'airports'
        PROJECTORG = 'apimgr'
        REGISTRY = 'ghcr.io'
        IMAGE_NAME = "${REGISTRY}/${PROJECTORG}/${PROJECTNAME}"

        // Version from release.txt or default
        VERSION = sh(script: 'cat release.txt 2>/dev/null || echo "0.0.1"', returnStdout: true).trim()
        COMMIT = sh(script: 'git rev-parse --short HEAD 2>/dev/null || echo "unknown"', returnStdout: true).trim()
        BUILD_DATE = sh(script: 'date -u +%Y-%m-%dT%H:%M:%SZ', returnStdout: true).trim()
    }

    stages {
        stage('Checkout') {
            agent { label 'amd64' }
            steps {
                checkout scm
                echo "Building ${PROJECTNAME} ${VERSION} (${COMMIT})"
            }
        }

        stage('Test') {
            parallel {
                stage('Test on AMD64') {
                    agent { label 'amd64' }
                    steps {
                        echo 'Running tests on AMD64...'
                        sh 'make test'
                    }
                }
                stage('Test on ARM64') {
                    agent { label 'arm64' }
                    steps {
                        echo 'Running tests on ARM64...'
                        sh 'make test'
                    }
                }
            }
        }

        stage('Build Binaries') {
            parallel {
                stage('Build on AMD64') {
                    agent { label 'amd64' }
                    steps {
                        echo 'Building binaries on AMD64...'
                        sh 'make build'
                        stash includes: 'binaries/**/*', name: 'binaries-amd64'
                        stash includes: 'releases/**/*', name: 'releases-amd64'
                    }
                }
                stage('Build on ARM64') {
                    agent { label 'arm64' }
                    steps {
                        echo 'Building binaries on ARM64...'
                        sh 'make build'
                        stash includes: 'binaries/**/*', name: 'binaries-arm64'
                        stash includes: 'releases/**/*', name: 'releases-arm64'
                    }
                }
            }
        }

        stage('Build Docker Images') {
            parallel {
                stage('Build Docker AMD64') {
                    agent { label 'amd64' }
                    steps {
                        echo 'Building Docker image for AMD64...'
                        script {
                            sh """
                                docker build \
                                    --platform linux/amd64 \
                                    --build-arg VERSION=${VERSION} \
                                    --build-arg COMMIT=${COMMIT} \
                                    --build-arg BUILD_DATE=${BUILD_DATE} \
                                    -t ${IMAGE_NAME}:${VERSION}-amd64 \
                                    -t ${IMAGE_NAME}:latest-amd64 \
                                    .
                            """
                        }
                    }
                }
                stage('Build Docker ARM64') {
                    agent { label 'arm64' }
                    steps {
                        echo 'Building Docker image for ARM64...'
                        script {
                            sh """
                                docker build \
                                    --platform linux/arm64 \
                                    --build-arg VERSION=${VERSION} \
                                    --build-arg COMMIT=${COMMIT} \
                                    --build-arg BUILD_DATE=${BUILD_DATE} \
                                    -t ${IMAGE_NAME}:${VERSION}-arm64 \
                                    -t ${IMAGE_NAME}:latest-arm64 \
                                    .
                            """
                        }
                    }
                }
            }
        }

        stage('Push Docker Images') {
            agent { label 'amd64' }
            when {
                anyOf {
                    branch 'main'
                    branch 'master'
                }
            }
            steps {
                echo 'Pushing Docker images to registry...'
                script {
                    withCredentials([usernamePassword(
                        credentialsId: 'github-registry',
                        usernameVariable: 'REGISTRY_USER',
                        passwordVariable: 'REGISTRY_TOKEN'
                    )]) {
                        sh """
                            echo \$REGISTRY_TOKEN | docker login ${REGISTRY} -u \$REGISTRY_USER --password-stdin

                            # Push architecture-specific images
                            docker push ${IMAGE_NAME}:${VERSION}-amd64
                            docker push ${IMAGE_NAME}:${VERSION}-arm64
                            docker push ${IMAGE_NAME}:latest-amd64
                            docker push ${IMAGE_NAME}:latest-arm64

                            # Create and push multi-arch manifest
                            docker manifest create ${IMAGE_NAME}:${VERSION} \
                                ${IMAGE_NAME}:${VERSION}-amd64 \
                                ${IMAGE_NAME}:${VERSION}-arm64

                            docker manifest create ${IMAGE_NAME}:latest \
                                ${IMAGE_NAME}:latest-amd64 \
                                ${IMAGE_NAME}:latest-arm64

                            docker manifest push ${IMAGE_NAME}:${VERSION}
                            docker manifest push ${IMAGE_NAME}:latest

                            docker logout ${REGISTRY}
                        """
                    }
                }
            }
        }

        stage('GitHub Release') {
            agent { label 'amd64' }
            when {
                anyOf {
                    branch 'main'
                    branch 'master'
                }
            }
            steps {
                echo 'Creating GitHub release...'
                script {
                    unstash 'releases-amd64'
                    withCredentials([string(
                        credentialsId: 'github-token',
                        variable: 'GITHUB_TOKEN'
                    )]) {
                        sh """
                            export GITHUB_TOKEN=\$GITHUB_TOKEN
                            make release
                        """
                    }
                }
            }
        }
    }

    post {
        success {
            echo "✅ Build successful for ${PROJECTNAME} ${VERSION}"
            echo "🐳 Docker images: ${IMAGE_NAME}:${VERSION}"
            echo "📦 Release: https://github.com/${PROJECTORG}/${PROJECTNAME}/releases/tag/${VERSION}"
        }
        failure {
            echo "❌ Build failed for ${PROJECTNAME} ${VERSION}"
        }
        always {
            cleanWs()
        }
    }
}
