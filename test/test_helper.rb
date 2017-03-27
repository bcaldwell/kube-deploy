$LOAD_PATH.unshift File.expand_path('../../lib', __FILE__)
require 'kube-deploy'

# require 'webmock/minitest'

# require 'vcr'

# VCR.configure do |config|
#   config.cassette_library_dir = "test/fixtures/vcr_cassettes"
#   config.hook_into :webmock
# end

# colors!!!
require 'minitest/reporters'
Minitest::Reporters.use!(Minitest::Reporters::SpecReporter.new)

require 'minitest/autorun'

require 'byebug'
