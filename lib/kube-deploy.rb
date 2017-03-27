require 'thor'
module KubeDeploy
  class CLI < Thor
    desc "deploy", "deploys configs from current folder to kubernetes cluster"
    def deploy
      def apply_file(file)
        system("kubectl", "apply", "-f", file)
      end

      def apply_files(files)
        folder = File.expand_path(File.dirname(__FILE__))
        extensions = %w(yml yaml)
        files.each do |file|
          extensions.each do |extension|
            config_file = File.join(folder, file) + ".#{extension}"
            next unless File.exist? config_file
            basename = File.basename(config_file)
            unless apply_file(config_file)
              puts "\x1b[31m✗\x1b[0m Failed to apply #{basename}"
              exit
            end
            puts "\x1b[32m✓\x1b[0m Applied #{basename}"
          end
        end
      end

      files = %w(namespace config gitlab-runner)
      apply_files(files)
    end

    default_task :deploy
  end
end
