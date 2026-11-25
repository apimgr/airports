pipeline {
    agent none

    triggers {
        // Devel build at 3am UTC (matches GitHub Actions devel.yml) and
        // devel devel image rebuild at 4am UTC (matches docker.yml build-devel job)
        cron('0 3 * * *\n0 4 * * *')
    }

    environment {
        PROJECT_NAME = 'airports'
        PROJECT_ORG = 'apimgr'
        INTERNAL_NAME = 'airports'
        BINDIR = 'binaries'
        RELDIR = 'releases'
        // Go cache bind-mounted from host: GO_CACHE (mod) and GO_BUILD (build cache)

        // =========================================================================
        // GIT PROVIDER CONFIGURATION
        // Uncomment ONE block below based on your git hosting platform
        // =========================================================================

        // ----- GITHUB (default) -----
        GIT_FQDN = 'github.com'
        // Jenkins credentials ID
        GIT_TOKEN = credentials('github-token')
        REGISTRY = "ghcr.io/${PROJECT_ORG}/${INTERNAL_NAME}"

        // ----- GITEA / FORGEJO (self-hosted) -----
        // GIT_FQDN = 'git.example.com'  // Your Gitea/Forgejo domain
        // GIT_TOKEN = credentials('gitea-token')  // Jenkins credentials ID
        // REGISTRY = "${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}"

        // ----- GITLAB (gitlab.com or self-hosted) -----
        // GIT_FQDN = 'gitlab.com'  // or your self-hosted GitLab domain
        // GIT_TOKEN = credentials('gitlab-token')  // Jenkins credentials ID
        // REGISTRY = "registry.${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}"

        // ----- DOCKER HUB -----
        // GIT_FQDN = 'github.com'  // Git provider (separate from registry)
        // GIT_TOKEN = credentials('github-token')
        // REGISTRY = "docker.io/${PROJECT_ORG}/${INTERNAL_NAME}"

        // =========================================================================
    }

    stages {
        stage('Setup') {
            agent { label 'amd64' }
            steps {
                script {
                    // Determine build type and version
                    if (env.TAG_NAME) {
                        // Release build (tag push) - matches release.yml
                        env.BUILD_TYPE = 'release'
                        env.VERSION = env.TAG_NAME.replaceFirst('^v', '')
                    } else if (env.BRANCH_NAME == 'beta') {
                        // Beta build - matches beta.yml
                        env.BUILD_TYPE = 'beta'
                        env.VERSION = sh(script: 'date -u +"%Y%m%d%H%M%S"', returnStdout: true).trim() + '-beta'
                    } else if (env.BRANCH_NAME == 'main' || env.BRANCH_NAME == 'master') {
                        // Devel build - matches devel.yml
                        // Devel identity is the commit being built, not a timestamp
                        env.BUILD_TYPE = 'devel'
                        env.VERSION = sh(script: 'git rev-parse --short=7 HEAD', returnStdout: true).trim()
                    } else {
                        // Other branches - dev build
                        env.BUILD_TYPE = 'dev'
                        env.VERSION = sh(script: 'date -u +"%Y%m%d%H%M%S"', returnStdout: true).trim() + '-dev'
                    }
                    env.COMMIT_ID = sh(script: 'git rev-parse --short=7 HEAD', returnStdout: true).trim()
                    env.BUILD_EPOCH = sh(script: 'date -u +%s', returnStdout: true).trim()
                    env.BUILD_DATE = sh(script: "date -u -d @${env.BUILD_EPOCH} +\"%Y-%m-%dT%H:%M:%SZ\"", returnStdout: true).trim()
                    // OFFICIAL_SITE (optional): site.txt wins; otherwise use Jenkins credentials or leave empty
                    // Never guess or assume - must be explicitly defined by user
                    env.OFFICIAL_SITE = sh(script: '[ -f site.txt ] && cat site.txt || echo "${OFFICIAL_SITE:-}"', returnStdout: true).trim()
                    env.LDFLAGS = "-s -w -X 'github.com/apimgr/airports/src/app.Version=${env.VERSION}' -X 'github.com/apimgr/airports/src/app.CommitID=${env.COMMIT_ID}' -X 'github.com/apimgr/airports/src/app.BuildEpoch=${env.BUILD_EPOCH}' -X 'github.com/apimgr/airports/src/app.OfficialSite=${env.OFFICIAL_SITE}'"
                    env.CLI_LDFLAGS = "-s -w -X 'main.Version=${env.VERSION}' -X 'main.CommitID=${env.COMMIT_ID}' -X 'main.BuildEpoch=${env.BUILD_EPOCH}' -X 'main.OfficialSite=${env.OFFICIAL_SITE}'"
                    env.HAS_CLI = sh(script: '[ -d src/client ] && echo true || echo false', returnStdout: true).trim()
                }
                sh 'mkdir -p ${BINDIR} ${RELDIR}'
                sh '''
                    docker run --rm \
                        --name "${PROJECT_NAME}-openapi-gen" \
                        -v ${WORKSPACE}:/app \
                        -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                        -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                        -w /app \
                        -e CGO_ENABLED=0 \
                        -e GOFLAGS=-buildvcs=false \
                        casjaysdev/go:latest \
                        sh -c 'GOOS=linux GOARCH=amd64 go run ./src/swagger/cmd/openapi-gen'
                '''
                echo "Build type: ${BUILD_TYPE}, Version: ${VERSION}"
            }
        }

        stage('Build') {
            parallel {
                // Linux
                stage('Linux AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=linux \
                                -e GOARCH=amd64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-linux-amd64 ./src
                        '''
                    }
                }
                stage('Linux ARM64') {
                    agent { label 'arm64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=linux \
                                -e GOARCH=arm64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-linux-arm64 ./src
                        '''
                    }
                }
                // Darwin (macOS)
                stage('Darwin AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=darwin \
                                -e GOARCH=amd64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-darwin-amd64 ./src
                        '''
                    }
                }
                stage('Darwin ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=darwin \
                                -e GOARCH=arm64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-darwin-arm64 ./src
                        '''
                    }
                }
                // Windows
                stage('Windows AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=windows \
                                -e GOARCH=amd64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-windows-amd64.exe ./src
                        '''
                    }
                }
                stage('Windows ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=windows \
                                -e GOARCH=arm64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-windows-arm64.exe ./src
                        '''
                    }
                }
                // FreeBSD
                stage('FreeBSD AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=freebsd \
                                -e GOARCH=amd64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-freebsd-amd64 ./src
                        '''
                    }
                }
                stage('FreeBSD ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=freebsd \
                                -e GOARCH=arm64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-freebsd-arm64 ./src
                        '''
                    }
                }
            }
        }

        // CLI builds - only if src/client/ exists (matches GitHub Actions)
        stage('Build CLI') {
            when {
                expression { env.HAS_CLI == 'true' }
            }
            parallel {
                stage('CLI Linux AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=linux \
                                -e GOARCH=amd64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${CLI_LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-cli-linux-amd64 ./src/client
                        '''
                    }
                }
                stage('CLI Linux ARM64') {
                    agent { label 'arm64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=linux \
                                -e GOARCH=arm64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${CLI_LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-cli-linux-arm64 ./src/client
                        '''
                    }
                }
                stage('CLI Darwin AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=darwin \
                                -e GOARCH=amd64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${CLI_LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-cli-darwin-amd64 ./src/client
                        '''
                    }
                }
                stage('CLI Darwin ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=darwin \
                                -e GOARCH=arm64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${CLI_LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-cli-darwin-arm64 ./src/client
                        '''
                    }
                }
                stage('CLI Windows AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=windows \
                                -e GOARCH=amd64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${CLI_LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-cli-windows-amd64.exe ./src/client
                        '''
                    }
                }
                stage('CLI Windows ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=windows \
                                -e GOARCH=arm64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${CLI_LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-cli-windows-arm64.exe ./src/client
                        '''
                    }
                }
                stage('CLI FreeBSD AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=freebsd \
                                -e GOARCH=amd64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${CLI_LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-cli-freebsd-amd64 ./src/client
                        '''
                    }
                }
                stage('CLI FreeBSD ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                                -v ${WORKSPACE}:/app \
                                -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                                -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                                -w /app \
                                -e CGO_ENABLED=0 \
                                -e GOOS=freebsd \
                                -e GOARCH=arm64 \
                                casjaysdev/go:latest \
                                go build -buildvcs=false -trimpath -ldflags "${CLI_LDFLAGS}" -o ${BINDIR}/${PROJECT_NAME}-cli-freebsd-arm64 ./src/client
                        '''
                    }
                }
            }
        }

        stage('Test') {
            agent { label 'amd64' }
            steps {
                sh '''
                    docker run --rm \
                        --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                        -v ${WORKSPACE}:/app \
                        -v ${GO_CACHE:-$HOME/go/pkg/mod}:/usr/local/share/go/pkg/mod \
                        -v ${GO_BUILD:-$HOME/.cache/go-build/${PROJECT_NAME}}:/usr/local/share/go/cache \
                        -w /app \
                        casjaysdev/go:latest \
                        go test -v -cover ./...
                '''
            }
        }

        stage('Security') {
            agent { label 'amd64' }
            steps {
                sh '''
                    docker run --rm \
                        --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                        -v ${WORKSPACE}:/app \
                        -w /app \
                        -e CGO_ENABLED=0 \
                        casjaysdev/go:latest \
                        govulncheck ./...
                '''
                sh '''
                    docker run --rm \
                        --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                        -v ${WORKSPACE}:/repo \
                        -w /repo \
                        trufflesecurity/trufflehog:latest \
                        git file:///repo --results=verified --exclude-paths=.github/trufflehog-exclude.txt --fail
                '''
            }
        }

        // Stable Release - matches release.yml (tag push only)
        stage('Release: Stable') {
            agent { label 'amd64' }
            when {
                expression { env.BUILD_TYPE == 'release' }
            }
            steps {
                sh '''
                    echo "${VERSION}" > ${RELDIR}/version.txt

                    for f in ${BINDIR}/${PROJECT_NAME}-*; do
                        [ -f "$f" ] || continue
                        cp "$f" ${RELDIR}/
                    done

                    tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
                        --exclude='.forgejo' --exclude='binaries' --exclude='releases' \
                        --exclude='*.tar.gz' \
                        -czf ${RELDIR}/${PROJECT_NAME}-${VERSION}-source.tar.gz .
                '''
                // SBOM tooling lives in the build image - never install it on the agent
                sh '''
                    docker run --rm \
                        --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                        -v ${WORKSPACE}:/app \
                        -w /app \
                        -e GOFLAGS=-buildvcs=false \
                        casjaysdev/go:latest \
                        cyclonedx-gomod mod -json -output "${RELDIR}/${PROJECT_NAME}-${VERSION}-sbom.cdx.json"
                '''
                sh '''
                    cd ${RELDIR}
                    FILES="$(ls)"
                    sha256sum $FILES > sha256.txt
                    sha512sum $FILES > sha512.txt
                '''
                archiveArtifacts artifacts: 'releases/*', fingerprint: true
            }
        }

        // Beta Release - matches beta.yml (beta branch only)
        stage('Release: Beta') {
            agent { label 'amd64' }
            when {
                expression { env.BUILD_TYPE == 'beta' }
            }
            steps {
                sh '''
                    echo "${VERSION}" > ${RELDIR}/version.txt

                    for f in ${BINDIR}/${PROJECT_NAME}-*; do
                        [ -f "$f" ] || continue
                        cp "$f" ${RELDIR}/
                    done

                    tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
                        --exclude='.forgejo' --exclude='binaries' --exclude='releases' \
                        --exclude='*.tar.gz' \
                        -czf ${RELDIR}/${PROJECT_NAME}-${VERSION}-source.tar.gz .
                '''
                // SBOM tooling lives in the build image - never install it on the agent
                sh '''
                    docker run --rm \
                        --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                        -v ${WORKSPACE}:/app \
                        -w /app \
                        -e GOFLAGS=-buildvcs=false \
                        casjaysdev/go:latest \
                        cyclonedx-gomod mod -json -output "${RELDIR}/${PROJECT_NAME}-${VERSION}-sbom.cdx.json"
                '''
                sh '''
                    cd ${RELDIR}
                    FILES="$(ls)"
                    sha256sum $FILES > sha256.txt
                    sha512sum $FILES > sha512.txt
                '''
                archiveArtifacts artifacts: 'releases/*', fingerprint: true
            }
        }

        // Devel Release - matches devel.yml (main/master + scheduled)
        stage('Release: Devel') {
            agent { label 'amd64' }
            when {
                expression { env.BUILD_TYPE == 'devel' }
            }
            steps {
                sh '''
                    echo "${VERSION}" > ${RELDIR}/version.txt

                    for f in ${BINDIR}/${PROJECT_NAME}-*; do
                        [ -f "$f" ] || continue
                        cp "$f" ${RELDIR}/
                    done

                    tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
                        --exclude='.forgejo' --exclude='binaries' --exclude='releases' \
                        --exclude='*.tar.gz' \
                        -czf ${RELDIR}/${PROJECT_NAME}-${VERSION}-source.tar.gz .
                '''
                // SBOM tooling lives in the build image - never install it on the agent
                sh '''
                    docker run --rm \
                        --name "${PROJECT_NAME}-$(tr -dc 'a-z0-9' </dev/urandom | head -c8)" \
                        -v ${WORKSPACE}:/app \
                        -w /app \
                        -e GOFLAGS=-buildvcs=false \
                        casjaysdev/go:latest \
                        cyclonedx-gomod mod -json -output "${RELDIR}/${PROJECT_NAME}-${VERSION}-sbom.cdx.json"
                '''
                sh '''
                    cd ${RELDIR}
                    FILES="$(ls)"
                    sha256sum $FILES > sha256.txt
                    sha512sum $FILES > sha512.txt
                '''
                archiveArtifacts artifacts: 'releases/*', fingerprint: true
            }
        }

        // Docker - matches docker.yml (ALL branches and tags)
        stage('Docker') {
            agent { label 'amd64' }
            steps {
                script {
                    def tags = "-t ${REGISTRY}:${COMMIT_ID}"

                    if (env.BUILD_TYPE == 'release') {
                        // Release tag - version, latest, YYMM
                        def yymm = new Date().format('yyMM')
                        tags += " -t ${REGISTRY}:${VERSION}"
                        tags += " -t ${REGISTRY}:latest"
                        tags += " -t ${REGISTRY}:${yymm}"
                    } else if (env.BUILD_TYPE == 'beta') {
                        // Beta branch - beta
                        tags += " -t ${REGISTRY}:beta"
                    }
                    // else: all other branches get only the {commit_id} tag
                    // above — :devel is built separately by the "Docker: Devel"
                    // stage below, never by this stage

                    // Login to container registry
                    // Works with: ghcr.io, registry.gitlab.com, gitea/forgejo, docker.io
                    sh """
                        echo "\${GIT_TOKEN}" | docker login ${REGISTRY.split('/')[0]} -u ${PROJECT_ORG} --password-stdin
                    """

                    // Build multi-arch with OCI labels and manifest annotations
                    sh """
                        docker buildx create --name ${PROJECT_NAME}-builder --use 2>/dev/null || docker buildx use ${PROJECT_NAME}-builder
                        docker buildx build \
                            -f docker/Dockerfile \
                            --platform linux/amd64,linux/arm64 \
                            --build-arg VERSION="${VERSION}" \
                            --build-arg COMMIT_ID="${COMMIT_ID}" \
                            --build-arg OFFICIAL_SITE="${OFFICIAL_SITE}" \
                            --build-arg BUILD_DATE="${BUILD_DATE}" \
                            --build-arg BUILD_EPOCH="${BUILD_EPOCH}" \
                            --label "org.opencontainers.image.vendor=${PROJECT_ORG}" \
                            --label "org.opencontainers.image.authors=${PROJECT_ORG}" \
                            --label "org.opencontainers.image.title=${PROJECT_NAME}" \
                            --label "org.opencontainers.image.base.name=${PROJECT_NAME}" \
                            --label "org.opencontainers.image.description=${PROJECT_NAME} - standard image (alpine)" \
                            --label "org.opencontainers.image.licenses=MIT" \
                            --label "org.opencontainers.image.version=${VERSION}" \
                            --label "org.opencontainers.image.created=${BUILD_DATE}" \
                            --label "org.opencontainers.image.revision=${COMMIT_ID}" \
                            --label "org.opencontainers.image.url=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --label "org.opencontainers.image.source=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --label "org.opencontainers.image.documentation=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --annotation "manifest:org.opencontainers.image.vendor=${PROJECT_ORG}" \
                            --annotation "manifest:org.opencontainers.image.authors=${PROJECT_ORG}" \
                            --annotation "manifest:org.opencontainers.image.title=${PROJECT_NAME}" \
                            --annotation "manifest:org.opencontainers.image.base.name=${PROJECT_NAME}" \
                            --annotation "manifest:org.opencontainers.image.description=${PROJECT_NAME} - standard image (alpine)" \
                            --annotation "manifest:org.opencontainers.image.licenses=MIT" \
                            --annotation "manifest:org.opencontainers.image.version=${VERSION}" \
                            --annotation "manifest:org.opencontainers.image.created=${BUILD_DATE}" \
                            --annotation "manifest:org.opencontainers.image.revision=${COMMIT_ID}" \
                            --annotation "manifest:org.opencontainers.image.url=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --annotation "manifest:org.opencontainers.image.source=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --annotation "manifest:org.opencontainers.image.documentation=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            ${tags} \
                            --push \
                            .
                    """

                    sh """
                        docker pull ${REGISTRY}:${COMMIT_ID}
                        DOCKER_TEST_IMAGE=${REGISTRY}:${COMMIT_ID} bash tests/docker.sh
                    """
                }
            }
        }

        // Docker Devel - matches docker.yml build-devel job (main/master push + daily cron, never the main Docker stage)
        stage('Docker: Devel') {
            agent { label 'amd64' }
            when {
                anyOf { branch 'main'; branch 'master' }
            }
            steps {
                script {
                    // Runs on every push to main/master (this pipeline's global cron trigger above
                    // also fires it devel) and always builds Dockerfile.dev with a
                    // single static :devel tag - MODE=development is baked into
                    // Dockerfile.dev via ENV, no extra build-arg needed here.
                    sh """
                        echo "\${GIT_TOKEN}" | docker login ${REGISTRY.split('/')[0]} -u ${PROJECT_ORG} --password-stdin
                    """

                    sh """
                        docker buildx create --name ${PROJECT_NAME}-devel-builder --use 2>/dev/null || docker buildx use ${PROJECT_NAME}-devel-builder
                        docker buildx build \
                            -f docker/Dockerfile.dev \
                            --platform linux/amd64,linux/arm64 \
                            --build-arg COMMIT_ID="${COMMIT_ID}" \
                            --build-arg OFFICIAL_SITE="${OFFICIAL_SITE}" \
                            --build-arg BUILD_DATE="${BUILD_DATE}" \
                            --build-arg BUILD_EPOCH="${BUILD_EPOCH}" \
                            --label "org.opencontainers.image.vendor=${PROJECT_ORG}" \
                            --label "org.opencontainers.image.authors=${PROJECT_ORG}" \
                            --label "org.opencontainers.image.title=${PROJECT_NAME}" \
                            --label "org.opencontainers.image.base.name=${PROJECT_NAME}" \
                            --label "org.opencontainers.image.description=${PROJECT_NAME} - development image (alpine, debug mode)" \
                            --label "org.opencontainers.image.licenses=MIT" \
                            --label "org.opencontainers.image.created=${BUILD_DATE}" \
                            --label "org.opencontainers.image.revision=${COMMIT_ID}" \
                            --label "org.opencontainers.image.url=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --label "org.opencontainers.image.source=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --label "org.opencontainers.image.documentation=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --annotation "manifest:org.opencontainers.image.vendor=${PROJECT_ORG}" \
                            --annotation "manifest:org.opencontainers.image.authors=${PROJECT_ORG}" \
                            --annotation "manifest:org.opencontainers.image.title=${PROJECT_NAME}" \
                            --annotation "manifest:org.opencontainers.image.base.name=${PROJECT_NAME}" \
                            --annotation "manifest:org.opencontainers.image.description=${PROJECT_NAME} - development image (alpine, debug mode)" \
                            --annotation "manifest:org.opencontainers.image.licenses=MIT" \
                            --annotation "manifest:org.opencontainers.image.created=${BUILD_DATE}" \
                            --annotation "manifest:org.opencontainers.image.revision=${COMMIT_ID}" \
                            --annotation "manifest:org.opencontainers.image.url=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --annotation "manifest:org.opencontainers.image.source=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            --annotation "manifest:org.opencontainers.image.documentation=https://${GIT_FQDN}/${PROJECT_ORG}/${INTERNAL_NAME}" \
                            -t ${REGISTRY}:devel \
                            --push \
                            .
                    """

                    sh """
                        docker pull ${REGISTRY}:devel
                        DOCKER_TEST_IMAGE=${REGISTRY}:devel bash tests/docker.sh
                    """
                }
            }
        }

    }

    post {
        always {
            cleanWs()
        }
    }
}
