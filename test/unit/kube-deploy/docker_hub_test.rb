require 'test_helper'
require "kube-deploy/docker_hub"

class DockerHubTest < Minitest::Test
  def setup
    @docker_hub = KubeDeploy::Docker::Hub.new
  end

  def test_repository
    assert_equal @docker_hub.repository("ubuntu"), @docker_hub.repository("library", "ubuntu")
  end
  def test_api
    # VCR.use_cassette("docker_hub_home") do
      assert_equal nil, @docker_hub.api_request()
    # end
  end

end
