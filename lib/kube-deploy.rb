require 'thor'
require 'yaml'

require 'helpers/printer'
require 'helpers/shell_runner'

require 'byebug'

module KubeDeploy
  class CLI < Thor
    desc "apply", "deploys configs from current folder to kubernetes cluster"
    method_option :config, :type => :string, :aliases => "-f"
    def apply
      pwd = Dir.pwd
      config = if options.config?
        config_file = File.join(pwd, options.config)
        unless File.exist? config_file
          Printer.puts_failure("#{options.config} does not exist")
          exit 1
        end
        begin
          YAML::load_file(config_file)
        rescue Exception => e  
          Printer.puts_failure("Unable to parse #{config}: #{e.message}")
        end
      else
        {
          "apply" => {
            "1" => Dir["*.yaml"] + Dir["*.yml"]
          }
        }
      end

      ShellRunner.run "Running pre script", config["pre-script"]

      config["apply"].each do |stage, files|
        Printer.put_header("Applying stage #{stage}")

        successful = true
        extensions = %w(.yml .yaml).unshift("")

        files.each do |file|
          break unless successful

          extensions.each do |extension|
            config_file = File.join(pwd, file) + "#{extension}"
            next unless File.exist? config_file
            basename = File.basename(config_file)
            unless system("kubectl", "apply", "-f", config_file)
              puts "\x1b[31m✗\x1b[0m Failed to apply #{basename}"
              successful = false
              break
            end
            puts "\x1b[32m✓\x1b[0m Applied #{basename}"
          end
        end
        Printer.put_footer(successful)
        exit 1 unless successful 

        ShellRunner.run "Running post stage script (#{stage})", config["post-stage"]
      end

      ShellRunner.run "Running post script", config["post-script"]
    end

    desc "version", "displays installed version"
    def version
      puts KubeDeploy::VERSION
    end

    default_task :apply
  end
end
