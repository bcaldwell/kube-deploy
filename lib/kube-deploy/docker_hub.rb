# https://success.docker.com/Cloud/Solve/How_do_I_authenticate_with_the_V2_API%3F
require 'net/http'
require 'json'

module KubeDeploy
  module Docker
    class Hub
      # def initialize(username = nil, password = nil)
      #   @username = username
      #   @password = password
      # end
      def repository(namespace = "library", project)
        api_request("repositories/#{namespace}/#{project}")
      end

      def api_request(path = "")
        url = URI.join("https://hub.docker.com/v2/", path)
        req = Net::HTTP::Get.new(url)
        # # req["Content-Type"] = "application/json"
        res = Net::HTTP.start(url.hostname, url.port, use_ssl: true) do |http|
          http.request(req)
        end

        return nil unless res.code == "200"
        JSON.parse(res.body)
      end
    end
  end
end
