module ShellRunner
  def self.run(title, commands)
    return unless commands&.any?
    Printer.put_header(title)
    successful = true
    commands.each do |command|
      system(command)
      if $?.exitstatus != 0
        successful = false
        Printer.puts_failure("#{command} returned non-zero status code: #{$?.exitstatus}")
        break
      end
    end
    Printer.put_footer(successful)
    exit 1 unless successful
  end
end