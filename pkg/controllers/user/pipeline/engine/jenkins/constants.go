package jenkins

const (
	CreateJobURI          = "/createItem"
	UpdateJobURI          = "/job/%s/config.xml"
	StopJobURI            = "/job/%s/%d/stop"
	CancelQueueItemURI    = "/queue/cancelItem?id=%d"
	DeleteBuildURI        = "/job/%s/%d/doDelete"
	GetCrumbURI           = "/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,\":\",//crumb)"
	JenkinsJobBuildURI    = "/job/%s/build"
	JenkinsJobInfoURI     = "/job/%s/api/json"
	JenkinsSetCredURI     = "/credentials/store/system/domain/_/createCredentials"
	JenkinsGetCredURI     = "/credentials/store/system/domain/_/credential/%s/api/json"
	JenkinsDeleteCredURI  = "/credentials/store/system/domain/_/credential/%s/doDelete"
	JenkinsBuildInfoURI   = "/job/%s/%d/api/json"
	JenkinsWFBuildInfoURI = "/job/%s/lastBuild/wfapi"
	JenkinsWFNodeInfoURI  = "/job/%s/lastBuild/execution/node/%s/wfapi"
	JenkinsWFNodeLogURI   = "/job/%s/lastBuild/execution/node/%s/wfapi/log"
	JenkinsBuildLogURI    = "/job/%s/%d/timestamps/?elapsed=HH'h'mm'm'ss's'S'ms'&appendLog"
	ScriptURI             = "/scriptText"
)

const stepFinishScript = `def result = manager.build.result
def command =  ["sh","-c","curl -s -d '' 'pipeline-server:60080/v1/events/stepfinish?id=%v&status=${result}&stageOrdinal=%v&stepOrdinal=%v'"]
manager.listener.logger.println command.execute().text`

const stepSCMFinishScript = `def result = manager.build.result
def env = manager.build.environment
def GIT_COMMIT = env.get("GIT_COMMIT")
def GIT_URL = env.get("GIT_URL")
def GIT_BRANCH = env.get("GIT_BRANCH")
def command =  ["sh","-c","curl -s -d 'GIT_URL=${GIT_URL}&GIT_BRANCH=${GIT_BRANCH}&GIT_COMMIT=${GIT_COMMIT}' 'pipeline-server:60080/v1/events/stepfinish?id=%v&status=${result}&stageOrdinal=%v&stepOrdinal=%v'"]
manager.listener.logger.println command.execute().text`

const stepStartScript = "curl -s -d '' 'pipeline-server:60080/v1/events/stepstart?id=%v&stageOrdinal=%v&stepOrdinal=%v'"

const PipelineJobScript = `
//Lets define a unique label for this build.
    def label = "buildpod.${env.JOB_NAME}.${env.BUILD_NUMBER}".replace('-', '_').replace('/', '_')

    //Lets create a new pod template with jnlp and maven containers, that uses that label.
    podTemplate(label: label, containers: [
            containerTemplate(name: 'maven', image: 'maven', ttyEnabled: true, command: 'cat'),
            containerTemplate(name: 'golang', image: 'golang:1.6.3-alpine', ttyEnabled: true, command: 'cat'),
            containerTemplate(name: 'jnlp', image: 'jenkinsci/jnlp-slave:alpine', envVars: [
                envVar(key: 'JENKINS_URL', value: 'http://jenkins:8080')], args: '${computer.jnlpmac} ${computer.name}', ttyEnabled: false)]) {

        //Lets use pod template (refernce by label)
        node(label) {
          timestamps {
            git 'https://github.com/gitlawr/php'
            parallel firstBranch: {
              stage('Genearate JSON schema'){
                container(name: 'golang') {
                  sh """
                    echo hiingolang && sleep 2 && echo donegolang
                  """
                }
              }
            }, secondBranch: {
              stage('Build model from JSON schema'){
                steps {
                    container(name: 'maven') {
                      parallel (
                        "s2": {echo 'Hello World 2'},
                        "s1": {echo 'Hello World'}
                        //"s4": {sh 'echo lalala'},
                        //"s3": {sh 'echo hiinmaven && sleep 10 && echo donemaven'}
                      )
                    }

                    container(name: 'maven') {
                      parallel (
                        //"s2": {echo 'Hello World 2'},
                        //"s1": {echo 'Hello World'},
                        "s4": {sh 'echo lalala'},
                        "s3": {sh 'echo hiinmaven && sleep 10 && echo donemaven'}
                      )
                    }
                }
              }
            }
          }
        }
    }
`
